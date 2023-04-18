package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func testOverlap(seed int64, N int, common int) int {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
	e := lt.NewEncoder[microbenchmarks.Transaction](rand.New(rand.NewSource(seed)), microbenchmarks.TestKey, dist, N)
	d := lt.NewDecoder[microbenchmarks.Transaction](microbenchmarks.TestKey, 2147483647)

	for i := 0; i < common; i++ {
		tx := microbenchmarks.GetTransaction(uint64(i))
		e.AddTransaction(tx)
		d.AddTransaction(tx)
	}
	toDecode := make(map[uint64]struct{})
	for i := common; i < N; i++ {
		tx := microbenchmarks.GetTransaction(uint64(i))
		e.AddTransaction(tx)
		toDecode[uint64(i)] = struct{}{}
	}
	Ncw := 0
	for len(toDecode) > 0 {
		c := e.ProduceCodeword()
		Ncw += 1
		_, newtx := d.AddCodeword(c)
		for _, tx := range newtx {
			delete(toDecode, tx.Data().Idx)
		}
	}
	return Ncw
}

func main() {
	fmt.Println("# block size 200")
	fmt.Println("# absOverlap  relOverlap  absCost  relCost")
	for overlap := 0; overlap < 200; overlap++ {
		tot := 0
		for i := 0; i < 100; i++ {
			res := testOverlap(int64(i), 200, overlap)
			tot += res
		}
		fmt.Println(overlap, float64(overlap)/200, float64(tot)/100, float64(tot)/100/float64(200-overlap))
	}
}
