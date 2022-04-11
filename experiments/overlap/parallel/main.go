package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"math"
)

func testOverlap(N int, commonFrac float64) (Ntx, Ncw int) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(N), 0.03, 0.5)
	dist2 := soliton.NewRobustSoliton(rand.New(rand.NewSource(2)), uint64(N), 0.03, 0.5)
	e1 := ldpc.NewEncoder(experiments.TestKey, dist1, N)
	e2 := ldpc.NewEncoder(experiments.TestKey, dist2, N)
	d := ldpc.NewDecoder(experiments.TestKey, 2147483647)

	txset := make(map[ldpc.Transaction]struct{})
	nc := int(float64(N) * commonFrac)
    nd := N - nc

	for i := 0; i < nc; i++ {
		tx := experiments.RandomTransaction()
		txset[*tx] = struct{}{}
		e1.AddTransaction(tx)
		e2.AddTransaction(tx)
	}
	for i := 0; i < nd; i++ {
		tx := experiments.RandomTransaction()
		txset[*tx] = struct{}{}
		e1.AddTransaction(tx)
		tx = experiments.RandomTransaction()
		txset[*tx] = struct{}{}
		e2.AddTransaction(tx)
	}
	ntx := len(txset)
	ncw := 0
	for len(txset) > int(0.05*float64(ntx)) {
		c1 := e1.ProduceCodeword()
		c2 := e2.ProduceCodeword()
		_, newtx := d.AddCodeword(c1)
		for _, tx := range newtx {
			delete(txset, *tx)
		}
		_, newtx = d.AddCodeword(c2)
		for _, tx := range newtx {
			delete(txset, *tx)
		}
		ncw += 2
	}
	return ntx, ncw
}

func main() {
	fmt.Println("# overlap  mean inflation   stddev inflation")
	Ns := []int{50, 200}
	for i, N := range Ns {
		if i != 0 {
			// for gnuplot
			fmt.Println()
			fmt.Println()
		}
		fmt.Printf("\"k=%d\"\n", N)
		for overlap := 0.0; overlap <= 1.01; overlap += 0.05 {
			total := 0.0
			totalSq := 0.0
			ntest := 400
			for i := 0; i < ntest; i++ {
				ntx, ncw := testOverlap(N, overlap)
				rate := float64(ncw) / float64(ntx)
				total += rate
				totalSq += rate * rate
			}
			avg := total / float64(ntest)
			stddev := math.Sqrt(totalSq / float64(ntest) - avg * avg)

			fmt.Printf("%.2f %.2f %.2f\n", overlap, avg, stddev)
		}
	}
}
