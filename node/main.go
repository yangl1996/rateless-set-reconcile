package main

import (
	"runtime"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"log"
	"net"
	"math/rand"
	"time"
	"io"
)

func randomTransaction() *ldpc.Transaction {
    d := ldpc.TransactionData{}
    rand.Read(d[:])
    t := &ldpc.Transaction{}
    t.UnmarshalBinary(d[:])
    return t
}

func main() {
	newController := func() *controller {
		return &controller {
			newPeerConn: make(chan io.ReadWriter),
			decodedTransaction: make(chan *ldpc.Transaction, 1000),
			localTransaction: make(chan *ldpc.Transaction, 1000),
		}
	}
	c1 := newController()
	c2 := newController()
	go c1.loop()
	go c2.loop()
	l, _ := net.Listen("tcp", ":9999")
	go func() {
		cn, _ := l.Accept()
		c1.newPeerConn <- cn
	}()
	go func() {
		cn, _ := net.Dial("tcp", ":9999")
		c2.newPeerConn <- cn
	}()

	go func() {
		for {
			tx := randomTransaction()
			c1.localTransaction <- tx
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
