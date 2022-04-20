package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"math"
)

func testOverlap(K int, txs []*ldpc.Transaction) (float64, float64) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)
	e1 := ldpc.NewEncoder(experiments.TestKey, dist1, K)
	d1 := ldpc.NewDecoder(experiments.TestKey, 2147483647)

	ptr := 0
	txset := make(map[ldpc.Transaction]struct{})
	for i := 0; i < K; i++ {
		e1.AddTransaction(txs[ptr])
		txset[*txs[ptr]] = struct{}{}
		ptr+=1
	}
	cnt1 := 0
	for len(txset) > 0 {
		c := e1.ProduceCodeword()
		_, newtx := d1.AddCodeword(c)
		for _, tx := range newtx {
			delete(txset, *tx.Transaction)
		}
		cnt1++
	}
	rate1 := float64(cnt1) / float64(K)

	nc := 5000
	ncode := 0
	max := 2.0
	min := 1.0
	test := func(rate2 float64) bool {
		ncode = 0
		txset2 := make(map[ldpc.Transaction]struct{})
		d2 := ldpc.NewDecoder(experiments.TestKey, 2147483647)
		e := ldpc.NewEncoder(experiments.TestKey, dist1, K)
		for i := 0; i < K; i++ {
			d2.AddTransaction(txs[i])
			d2.AddTransaction(txs[i+K+nc])
			//e.AddTransaction(txlist[i])
		}
		for i := K; i < nc+K; i++ {
			txset2[*txs[i]] = struct{}{}
		}
		credit := 0.0
		for i := 0; i < nc+K+K; i++ {
			e.AddTransaction(txs[i])
			if i < K {
				continue
			}
			credit += rate2
			for credit > 1.0 {
				c := e.ProduceCodeword()
				ncode += 1
				_, newtx := d2.AddCodeword(c)
				for _, tx := range newtx {
					delete(txset2, *tx.Transaction)
				}
				credit -= 1.0
				if len(txset2) == 0 {
					return true
				}
			}
		}
		if len(txset2) == 0 {
			return true
		}
		return false
	}
	for max-min > 0.01 {
		rate2 := (max+min)/2.0
		if test(rate2) {
			max = rate2
		} else {
			min = rate2
		}
	}
	return rate1,max
}

func main() {
	txs := []*ldpc.Transaction{}
	{
		dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(50), 0.03, 0.5)
		catchConf := ldpc.NewEncoder(experiments.TestKey, dist1, 100000)
		for len(txs) < 100000 {
			tx := experiments.RandomTransaction()
			ok := catchConf.AddTransaction(tx)
			if ok {
				txs = append(txs, tx)
			}
		}
	}
	fmt.Println("# K  conventional stddev windowed stddev")
	Ns := []int{20, 50, 75, 100, 150, 200}
	for _, N := range Ns {
		var normalTotal, normalTotalSq, windowTotal, windowTotalSq float64
		ntest := 100
		for i := 0; i < ntest; i++ {
			normal, window := testOverlap(N, txs)
			normalTotal += normal
			normalTotalSq += normal * normal
			windowTotal += window
			windowTotalSq += window * window
		}
		//avg := total / float64(ntest)
		//stddev := math.Sqrt(totalSq / float64(ntest) - avg * avg)

		avg := func(t float64) float64 {
			return t / float64(ntest)
		}
		normalAvg := avg(normalTotal)
		windowAvg := avg(windowTotal)
		normalStd := math.Sqrt(avg(normalTotalSq)-normalAvg*normalAvg)
		windowStd := math.Sqrt(avg(windowTotalSq)-windowAvg*windowAvg)
		fmt.Printf("%d %.2f %.2f %.2f %.2f\n", N, normalAvg, normalStd, windowAvg, windowStd)
	}
}
