package main

import (
	"strconv"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"os"
	"sync"
	"os/exec"
	"strings"
	"math/rand"
	"time"
)

type RemoteError struct {
	inner   error
	problem string
}

func (e RemoteError) Error() string {
	if e.inner != nil {
		return e.problem + ": " + e.inner.Error()
	} else {
		return e.problem
	}
}


func dispatchBwTest(args []string) {
	rand.Seed(time.Now().UnixNano())
	command := flag.NewFlagSet("exp", flag.ExitOnError)
	serverListFilePath := command.String("l", "servers.json", "path to the server list file")
	install := command.String("install", "", "install the given binary")
	runExp := command.String("run", "", "run the test with the given setup file")
	downloadResults := command.String("dl", "", "download the results and store it with the given prefix")
	measure := command.String("ping", "", "ping the nodes to get the latency using the given setup file")

	command.Parse(args[0:])

	if *serverListFilePath == "" {
		fmt.Println("missing server list")
		os.Exit(1)
	}

	// parse the server list
	servers := ReadServerInfo(*serverListFilePath)

	clients := make([]*ssh.Client, len(servers))
	connWg := &sync.WaitGroup{}	// wait for the ssh connection
	connWg.Add(len(servers))
	for i, s := range servers {
		go func(i int, s Server) {
			defer connWg.Done()
			client, err := connectSSH(s.User, s.PublicIP, s.Port, s.KeyPath)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Printf("Connected to %v\n", s.Location)
			clients[i] = client
		}(i, s)
	}
	connWg.Wait()

	if *install != "" {
		fn := func(i int, s Server, c *ssh.Client) error {
			if err := killServer(c); err != nil {
				return err
			}
			return uploadFile(s, *install, "txcode-node")
		}
		runAll(servers, clients, fn)
	}

	if *measure != "" {
		exp := ReadExperimentInfo(*measure)
		fn := func(i int, s Server, c *ssh.Client) error {
			for _, pair := range exp.Topology {
				if pair.From == i {
					cmd := fmt.Sprintf("ping -c 30 %s | tail -n1 | cut -f5 -d'/'", servers[pair.To].PublicIP)
					sess, err := c.NewSession()
					if err != nil {
						return err
					}
					defer sess.Close()
					out, err := sess.Output(cmd)
					if err != nil {
						return err
					}
					meanDelay, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
					if err != nil {
						return err
					}
					line := fmt.Sprintf("        node[%d].peer++ <--> {  delay = %.1fms; } <--> node[%d].peer++;", pair.From, meanDelay/2.0, pair.To)
					fmt.Println(line)
				}
			}
			return nil
		}
		runAll(servers, clients, fn)
	}

	if *runExp != "" {
		// port scanners send garbage data and confuses gob; randomize the port to mitigate

		port := int(rand.Float64() * 40000.0) + 10000
		exp := ReadExperimentInfo(*runExp)
		fn := func(i int, s Server, c *ssh.Client) error {
			// figure out my outgoing peers
			peerAddrs := []string{}
			for _, pair := range exp.Topology {
				if pair.From == i {
					peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", servers[pair.To].PublicIP, port))
				}
			}
			var peerCmd string
			if len(peerAddrs) > 0 {
				peerCmd = strings.Join(peerAddrs, ",")
			}
			if err := killServer(c); err != nil {
				return err
			}
			sess, err := c.NewSession()
			if err != nil {
				return err
			}
			defer sess.Close()
			cmd := "bash -c 'ufw disable ; nohup "
			if peerCmd != "" {
				cmd += fmt.Sprintf("./txcode-node -p %s", peerCmd)
			} else {
				cmd += fmt.Sprintf("./txcode-node")
			}
			cmd += fmt.Sprintf(" -l 0.0.0.0:%d %s > log.txt 2>&1 &'", port, strings.Join(command.Args(), " "))
			fmt.Println(s.Location, "started running")
			return sess.Run(cmd)
		}
		runAll(servers, clients, fn)
	}

	if *downloadResults != "" {
		fn := func(i int, s Server, c *ssh.Client) error {
			if err := killServer(c); err != nil {
				return err
			}
			return copyBackFile(s, "log.txt", fmt.Sprintf("%s-%d", *downloadResults, i))
		}
		runAll(servers, clients, fn)
	}
}


func runAll(servers []Server, clients []*ssh.Client, fn func(int, Server, *ssh.Client) error) error {
	if len(servers) != len(clients) {
		panic("incorrect")
	}
	wg := &sync.WaitGroup{}
	wg.Add(len(clients))
	for i := range clients {
		go func(i int, s Server, c *ssh.Client) {
			defer wg.Done()
			err := fn(i, s, c)
			if err != nil {
				switch err := err.(type) {
				case *exec.ExitError:
					fmt.Printf("error executing local command for server %v: %s\n", i, err.Stderr)
				case *ssh.ExitError:
					fmt.Printf("error executing command on server %v: %s\n", i, err.Msg())
				default:
					fmt.Printf("error executing on server %v: %v\n", i, err)
				}
			}
		}(i, servers[i], clients[i])
	}
	wg.Wait()
	return nil
}

// TODO: use go-native ssh
func copyBackFile(s Server, from, dest string) error {
	fromStr := fmt.Sprintf("%s@%s:%s", s.User, s.PublicIP, from)
	cmdArgs := []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", s.KeyPath, fromStr, dest}
	proc := exec.Command("scp", cmdArgs...)
	err := proc.Run()
	if err != nil {
		return err
	}
	return nil
}

func uploadFile(s Server, from, dest string) error {
	toStr := fmt.Sprintf("%s@%s:%s", s.User, s.PublicIP, dest)
	cmdArgs := []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", s.KeyPath, from, toStr}
	//fmt.Println(cmdArgs)
	proc := exec.Command("scp", cmdArgs...)
	err := proc.Run()
	if err != nil {
		return err
	}
	return nil
}

func killServer(c *ssh.Client) error {
	pkill, err := c.NewSession()
	if err != nil {
		return RemoteError{err, "error creating session"}
	}
	pkill.Run(`killall -w txcode-node`)
	pkill.Close()
	return nil
}

