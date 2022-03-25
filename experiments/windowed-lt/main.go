package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"math"
)

func testOverlap(K int) (float64, float64) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)
	e1 := ldpc.NewEncoder(experiments.TestKey, dist1, K)
	d1 := ldpc.NewDecoder(experiments.TestKey, 2147483647)

	txset := make(map[ldpc.Transaction]struct{})
	for i := 0; i < K; i++ {
		tx := experiments.RandomTransaction()
		e1.AddTransaction(tx)
		txset[*tx] = struct{}{}
	}
	cnt1 := 0
	for len(txset) > 0 {
		c := e1.ProduceCodeword()
		_, newtx := d1.AddCodeword(c)
		for _, tx := range newtx {
			delete(txset, *tx)
		}
		cnt1++
	}
	rate1 := float64(cnt1) / float64(K)

	nc := 5000
	rate2 := 1.0
	txlist := []*ldpc.Transaction{}
	for i := 0; i < nc+2*K; i++ {
		tx := experiments.RandomTransaction()
		txlist = append(txlist, tx)
	}
	ncode := 0
	for ;;rate2 += 0.02 {
		ncode = 0
		if rate2 > 5.0 {
			panic("high rate")
		}
		txset2 := make(map[ldpc.Transaction]struct{})
		d2 := ldpc.NewDecoder(experiments.TestKey, 2147483647)
		e := ldpc.NewEncoder(experiments.TestKey, dist1, K)
		for i := 0; i < K; i++ {
			d2.AddTransaction(txlist[i])
			d2.AddTransaction(txlist[i+K+nc])
			//e.AddTransaction(txlist[i])
		}
		for i := K; i < nc+K; i++ {
			txset2[*txlist[i]] = struct{}{}
		}
		credit := 0.0
		for i := 0; i < nc+K+K; i++ {
			e.AddTransaction(txlist[i])
			if i < K {
				continue
			}
			credit += rate2
			for credit > 1.0 {
				c := e.ProduceCodeword()
				ncode += 1
				_, newtx := d2.AddCodeword(c)
				for _, tx := range newtx {
					delete(txset2, *tx)
				}
				credit -= 1.0
				if len(txset2) == 0 {
					break
				}
			}
		}
		if len(txset2) == 0 {
			break
		}
	}
	return rate1,rate2 
}

func main() {
	fmt.Println("# K  conventional stddev windowed stddev")
	Ns := []int{50, 100, 200}
	for _, N := range Ns {
		var normalTotal, normalTotalSq, windowTotal, windowTotalSq float64
		ntest := 100
		for i := 0; i < ntest; i++ {
			normal, window := testOverlap(N)
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
