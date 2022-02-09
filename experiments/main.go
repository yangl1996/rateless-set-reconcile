package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"math/rand"
)

type uniform struct{}

func (u uniform) Uint64() uint64 {
	return 1
}

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

func testDist(dist ldpc.DegreeDistribution, rate float64) float64{
	e := ldpc.NewEncoder(testKey, dist, 50)
	d := ldpc.NewDecoder(testKey)
	for i := 0; i < 50; i++ {
		tx := randomTransaction()
		e.AddTransaction(tx)
	}
	credit := 0.0
	factor := rate
	for i := 0; i < 1000000; i++ {
		tx := randomTransaction()
		e.AddTransaction(tx)
		credit += factor
		for credit > 1.0 {
			credit -= 1.0
			c := e.ProduceCodeword()
			d.AddCodeword(c)
		}
	}
	return float64(d.NumTransactionsReceived()) / float64(1000000)
}

func testUntil(r float64, d ldpc.DegreeDistribution) {
	rate := 1.0
	for {
		f := testDist(d, rate)
		fmt.Printf("%.2f, %.2f\n", rate, f*100)
		if f < r {
			rate += 0.1
		} else {
			break
		}
	}
}

func main() {
	fmt.Println("degree-1")
	testUntil(0.95, uniform{})
	fmt.Println("bimodal 0.2 0.8")
	testUntil(0.95, bimodal{0.2})
	fmt.Println("soliton")
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	testUntil(0.95, dist)
}
