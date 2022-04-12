package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func testOverlap(K, N int, overlap float64) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	// create 2N transactions
	txs := []*ldpc.Transaction{}
	{
		catchConf := ldpc.NewEncoder(experiments.TestKey, dist, N*2)
		for len(txs) < N*2 {
			tx := experiments.RandomTransaction()
			ok := catchConf.AddTransaction(tx)
			if ok {
				txs = append(txs, tx)
			}
		}
	}

	//minRate := (2.0-overlap)/2.0
	minRate := 0.0
	maxRate := 2.0
	//maxRate := 2.0-overlap
	for r1 := minRate; r1 <= maxRate; r1 += 0.05 {
		for r2 := minRate; r2 <= maxRate; r2 += 0.05 {
			d1 := ldpc.NewDecoder(experiments.TestKey, 2147483647)
			e1 := ldpc.NewEncoder(experiments.TestKey, dist, K)
			e2 := ldpc.NewEncoder(experiments.TestKey, dist, K)

			txset1 := make(map[ldpc.Transaction]struct{})
			txset2 := make(map[ldpc.Transaction]struct{})
			c1 := 0.0
			c2 := 0.0
			ptr := 0
			for i := 0; i < N; i++ {
				if (rand.Float64() < overlap) {
					e1.AddTransaction(txs[ptr])
					e2.AddTransaction(txs[ptr])
					txset1[*txs[ptr]] = struct{}{}
					txset2[*txs[ptr]] = struct{}{}
					ptr++
				} else {
					e1.AddTransaction(txs[ptr])
					txset1[*txs[ptr]] = struct{}{}
					ptr++
					e2.AddTransaction(txs[ptr])
					txset2[*txs[ptr]] = struct{}{}
					ptr++
				}
				c1 += r1
				c2 += r2
				for c1 >= 1.0 {
					c := e1.ProduceCodeword()
					_, newtx := d1.AddCodeword(c)
					for _, tx := range newtx {
						delete(txset1, *tx)
						delete(txset2, *tx)
					}
					c1 -= 1.0
				}
				for c2 >= 1.0 {
					c := e2.ProduceCodeword()
					_, newtx := d1.AddCodeword(c)
					for _, tx := range newtx {
						delete(txset1, *tx)
						delete(txset2, *tx)
					}
					c2 -= 1.0
				}
			}
			deliver1 := float64(N-len(txset1)) / float64(N)
			deliver2 := float64(N-len(txset2)) / float64(N)
			if deliver1 > 0.95 && deliver2 > 0.95 {
				fmt.Printf("%.2f %.2f %.2f %.2f\n", r1, r2, deliver1, deliver2)
				break
			}
		}
	}
}

func main() {
	fmt.Println("# rate1 rate2 deliver1 deliver2")
	testOverlap(50, 10000, 0.8)
}
