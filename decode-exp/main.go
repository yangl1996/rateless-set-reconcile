package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	_ "net/http/pprof"
	"net/http"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"
)

func main() {
	go http.ListenAndServe("localhost:6060", nil)
	flag.Parse()
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if cfg.Seed == 0 {
		cfg.Seed = time.Now().UTC().UnixNano()
	}
	rand.Seed(cfg.Seed)

	// create output files
	var f *os.File
	var pressureF *os.File
	var cwpoolF *os.File
	if *outputPrefix != "" {
		var err error
		err = writeConfigFile(*outputPrefix + ".cfg", &cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		f, err = os.Create(*outputPrefix + "-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintf(f, "# num decoded     symbols rcvd     ms for last 1k txs\n")

		pressureF, err = os.Create(*outputPrefix + "-ntx-unique-to-p1.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(pressureF, "# iteration     unique to P1\n")
		defer pressureF.Close()

		cwpoolF, err = os.Create(*outputPrefix + "-p2-codeword-pool.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(cwpoolF, "# iteration     P2 unreleased cw\n")
		defer cwpoolF.Close()
	}

	if *runtimetrace != "" {
		f, err := os.Create(*runtimetrace)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()

		if err := trace.Start(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer trace.Stop()
	}

	// start the profile
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	var chs []chan int    // channels for the interation-#decoded result
	var pressureChs []chan int // channel to collect num of undecoded transactions
	var cwpoolChs []chan int   // channel to collect the number of unreleased codewords
	procwg := &sync.WaitGroup{}
		procwg.Add(1)
		var ch chan int
		if f != nil {
			ch = make(chan int, 1000)
		}
		chs = append(chs, ch)
		var pressureCh chan int
		if pressureF != nil {
			pressureCh = make(chan int, 1000)
		}
		pressureChs = append(pressureChs, pressureCh)
		var cwpoolCh chan int
		if cwpoolF != nil {
			cwpoolCh = make(chan int, 1000)
		}
		cwpoolChs = append(cwpoolChs, cwpoolCh)
		sd := rand.Int63()
		go func(s int64) {
			if ch != nil {
				defer close(ch)
			}
			if pressureCh != nil {
				defer close(pressureCh)
			}
			if cwpoolCh != nil {
				defer close(cwpoolCh)
			}
			defer procwg.Done()
			err := runExperiment(cfg, ch, pressureCh, cwpoolCh)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}(sd)

	// monitor and dump to files
	wg := &sync.WaitGroup{}
	// for each tx idx, range over res channels to collect data and dump to file
	if f != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now().UnixMilli()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(chs, dch)
			for d := range dch {
				if f != nil {
					if idx % 1000 == 0 && idx != 0 {
						fmt.Fprintf(f, "%v        %v        %v\n", idx, d, time.Now().UnixMilli()-start)
						start = time.Now().UnixMilli()
					} else {
						fmt.Fprintf(f, "%v        %v\n", idx, d)
					}
				}
				fmt.Printf("Iteration=%v, transactions=%v\n", d, idx)
				idx += 1
			}
		}()
	}
	// collect and dump num of transactions unique to p1
	if pressureF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(pressureChs, dch)
			for d := range dch {
				fmt.Fprintf(pressureF, "%v        %v\n", idx, d)
				idx += 1
			}
		}()
	}
	if cwpoolF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(cwpoolChs, dch)
			for d := range dch {
				fmt.Fprintf(cwpoolF, "%v        %v\n", idx, d)
				idx += 1
			}
		}()
	}

	procwg.Wait()
	wg.Wait()

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return
}

type StuckError struct {
	nid int
}

func (e StuckError) Error() string {
	return fmt.Sprintf("node %v is stuck", e.nid)
}

type TransactionCountError struct {
	nid int
}

func (e TransactionCountError) Error() string {
	return fmt.Sprintf("transaction count limit reached at node %v", e.nid)
}

func runExperiment(cfg ExperimentConfig, res, diff, cwpool chan int) error {
	rng := rand.New(rand.NewSource(cfg.Seed))
	var nodes []*node
	nodeName := make(map[string]*node)
	for _, server := range cfg.Topology.Servers {
		p, err := newNode(server, rng)
		if err != nil {
			return err
		}
		nodeName[server.Name] = p
		nodes = append(nodes, p)
	}
	for _, cn := range cfg.Topology.Connections {
		nodeName[cn.Car].connectTo(nodeName[cn.Cdr])
	}
	totalTx := 0
	var sources []*txsource
	for _, source := range cfg.Topology.Sources {
		s, err := newSource(source, rng, nodeName)
		if err != nil {
			return err
		}
		sources = append(sources, s)
		totalTx += source.InitialTx
	}

	if res != nil {
		res <- 0 // at iteration 0, we have decoded 0 transactions
	}
	// start sending codewords
	// prepare the counters
	i := 0 // iteration counter
	for {
		i += 1
		for nidx := range nodes {
			nodes[nidx].sendCodewords()
		}
		for nidx := range nodes {
			updated := nodes[nidx].tryDecode()
			if nidx == 0 {
				if res != nil {
					for cnt := 0; cnt < updated; cnt++ {
						res <- nodes[nidx].cwrcvd
					}
				}
				if diff != nil {
					diff <- totalTx - nodes[nidx].txPoolSize()
				}
				if cwpool != nil {
					sum := 0
					for _, peer := range nodes[nidx].PeerStates {
						sum += peer.NumPendingCodewords()
					}
					cwpool <- sum
				}
			}
			// stop if node is stuck
			if int(nodes[nidx].Seq-nodes[nidx].lastAct) > nodes[nidx].timeout {
				return StuckError{nidx}
			}
			if nodes[nidx].declimit != 0 &&  nodes[nidx].declimit <= nodes[nidx].decoded {
				return TransactionCountError{nidx}
			}
		}
		// add transactions to pools
		for _, src := range sources {
			totalTx += src.generate()
		}
	}
}

func collectDumpHistogram(f *os.File, ch chan int) {
	hist := make(map[int]int)
	maxd := 0
	tot := 0
	totPrd := 0
	for d := range ch {
		hist[d] += 1
		if maxd < d {
			maxd = d
		}
		tot += 1
		totPrd += d
	}
	cdf := 0
	for i := 0; i <= maxd; i++ {
		if val, there := hist[i]; there {
			cdf += val * i
			fmt.Fprintf(f, "%v         %v           %v\n", i, float64(val)/float64(tot), float64(cdf)/float64(totPrd))
		}
	}
}

func collectAverage(chs []chan int, out chan int) {
	defer close(out)
	for {
		nClosed := 0
		d := 0
		for _, ch := range chs {
			td, more := <-ch
			if more {
				d += td
			} else {
				nClosed += 1
			}
		}
		if nClosed >= len(chs) {
			return
		} else if nClosed == 0 {
			out <- d / len(chs)
		}
	}
}
