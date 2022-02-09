package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"math/rand"
)

type bimodal struct {
	prob1 float64
}

func (b bimodal) Uint64() uint64 {
	if rand.Float64() < b.prob1 {
		return 1
	} else {
		return 2
	}
}

func randomTransaction() *ldpc.Transaction {
	d := ldpc.TransactionData{}
    rand.Read(d[:])
	t := &ldpc.Transaction{}
	t.UnmarshalBinary(d[:])
    return t
}

var testKey = [ldpc.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func main() {
	// test the encoder with simple degree distribution
	{
		//dist := bimodal{0.5}
		dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 200, 0.03, 0.5)
		e := ldpc.NewEncoder(testKey, dist, 200)
		d := ldpc.NewDecoder(testKey)
		for i := 0; i < 200; i++ {
			tx := randomTransaction()
			e.AddTransaction(tx)
		}
		credit := 0.0
		factor := 1.15
		for i := 0; i < 1000000; i++ {
			tx := randomTransaction()
			e.AddTransaction(tx)
			credit += factor
			for credit > 1.0 {
				credit -= 1.0
				c := e.ProduceCodeword()
				d.AddCodeword(c)
			}
			if i % 1000 == 0 {
				fmt.Printf("%d %d\n", i, d.NumTransactionsReceived())
			}
		}
	}
}
