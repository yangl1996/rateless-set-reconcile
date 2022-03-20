package main

import (
	"runtime"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"log"
	"net"
	"math/rand"
	"time"
	"io"
	"flag"
	"strings"
)

func randomTransaction() *ldpc.Transaction {
    d := ldpc.TransactionData{}
    rand.Read(d[:])
    t := &ldpc.Transaction{}
    t.UnmarshalBinary(d[:])
    return t
}

func main() {
	addr := flag.String("l", ":9000", "address to listen")
	conn := flag.String("c", "", "comma-delimited list of addresses to connect to")
	newController := func() *controller {
		return &controller {
			newPeerConn: make(chan io.ReadWriter),
			decodedTransaction: make(chan *ldpc.Transaction, 1000),
			localTransaction: make(chan *ldpc.Transaction, 1000),
		}
	}
	c := newController()

	go c.loop()
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalln("failed to listen to", *addr)
	}
	go func() {
		for {
			cn, err := l.Accept()
			if err != nil {
				log.Println("error accepting incoming connection")
			} else {
				c.newPeerConn <- cn
			}
		}
	}()

	if *conn != "" {
		for _, a := range strings.Split(*conn, ",") {
			go func() {
				cn, err := net.Dial("tcp", a)
				if err != nil {
					log.Println("error connecting to peer", a)
				} else {
					c.newPeerConn <- cn
				}
			}()
		}
	}

	go func() {
		for {
			tx := randomTransaction()
			c.localTransaction <- tx
			time.Sleep(time.Duration(1 * time.Millisecond))
		}
	}()

	log.Println("running")

	var m runtime.MemStats
	for {
		runtime.ReadMemStats(&m)
		log.Printf("Heap=%v MB, Sys=%v MB, GCCycles=%v\n", m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
		time.Sleep(1 * time.Second)
	}
}
