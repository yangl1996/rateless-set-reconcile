package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func testOverlap(T1, T2, Tc, K int) (c1, c2 int) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)
	dist2 := soliton.NewRobustSoliton(rand.New(rand.NewSource(2)), uint64(K), 0.03, 0.5)
	e1 := ldpc.NewEncoder(experiments.TestKey, dist1, T1+Tc)
	e2 := ldpc.NewEncoder(experiments.TestKey, dist2, T2+Tc)
	d := ldpc.NewDecoder(experiments.TestKey, 2147483647)

	txset1 := make(map[ldpc.Transaction]struct{})
	txset2 := make(map[ldpc.Transaction]struct{})

	cnt := 0
	for cnt < Tc {
		tx := experiments.RandomTransaction()
		if e1.AddTransaction(tx) {
			e2.AddTransaction(tx)
			txset1[*tx] = struct{}{}
			txset2[*tx] = struct{}{}
			cnt += 1
		}
	}
	cnt = 0
	for cnt < T1 {
		tx := experiments.RandomTransaction()
		if e1.AddTransaction(tx) {
			txset1[*tx] = struct{}{}
			cnt += 1
		}
	}
	cnt = 0
	for cnt < T1 {
		tx := experiments.RandomTransaction()
		if e2.AddTransaction(tx) {
			txset2[*tx] = struct{}{}
			cnt += 1
		}
	}

	cnt1 := 0
	cnt2 := 0
	for len(txset1) > int(0.02*float64(T1+Tc)) || len(txset2) > int(0.02*float64(T2+Tc)) {
		c1 := e1.ProduceCodeword()
		c2 := e2.ProduceCodeword()
		_, newtx := d.AddCodeword(c1)
		cnt1 += 1
		for _, tx := range newtx {
			delete(txset1, *tx.Transaction)
			delete(txset2, *tx.Transaction)
		}
		_, newtx = d.AddCodeword(c2)
		cnt2 += 1
		for _, tx := range newtx {
			delete(txset1, *tx.Transaction)
			delete(txset2, *tx.Transaction)
		}
	}
	return cnt1, cnt2
}

func main() {
	total1 := 0
	total2 := 0
	ntest := 10
	for i := 0; i < ntest; i++ {
		cnt1, cnt2 := testOverlap(20000, 20000, 100000, 10000)
		total1 += cnt1
		total2 += cnt2
	}
	avg1 := float64(total1) / float64(ntest) / float64(20000+100000)
	avg2 := float64(total2) / float64(ntest) / float64(20000+100000)

	fmt.Printf("sender1=%.2f sender2=%.2f\n", avg1, avg2)
}
