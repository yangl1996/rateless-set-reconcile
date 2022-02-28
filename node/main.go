package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"log"
	"net"
	"math/rand"
	"time"
)

func randomTransaction() *ldpc.Transaction {
    d := ldpc.TransactionData{}
    rand.Read(d[:])
    t := &ldpc.Transaction{}
    t.UnmarshalBinary(d[:])
    return t
}

func main() {
	c1 := newController()
	c2 := newController()
	go c1.loop()
	go c2.loop()
	l, _ := net.Listen("tcp", ":9999")
	go func() {
		cn, _ := l.Accept()
		c1.newPeer <- cn
	}()
	go func() {
		cn, _ := net.Dial("tcp", ":9999")
		c2.newPeer <- cn
	}()

	go func() {
		for {
			tx := randomTransaction()
			c1.localTransaction <- tx
			time.Sleep(time.Duration(1 * time.Millisecond))
			 
		}
	}()

	log.Println("running")

	select{}
}
