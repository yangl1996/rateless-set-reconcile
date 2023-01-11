package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
)

func testOverlap(rng *rand.Rand, t, k int) {
	dist := soliton.NewRobustSoliton(rng, uint64(k), 0.03, 0.5)
	e := ldpc.NewEncoder(experiments.TestKey, dist, t)
	d := ldpc.NewDecoder(experiments.TestKey, 2147483647)

	txset := make(map[ldpc.Transaction]struct{})

	cnt := 0
	for cnt < t {
		tx := experiments.RandomTransaction()
		if e.AddTransaction(tx) {
			txset[*tx] = struct{}{}
			cnt += 1
		}
	}

	n := 0
	orig := len(txset)
	for len(txset) > 0 {
		cw := e.ProduceCodeword()
		_, newtx := d.AddCodeword(cw)
		for _, tx := range newtx {
			delete(txset, *tx.Transaction)
		}
		n += 1
		fmt.Println(n, orig-len(txset))
	}
}

func main() {
	rng := rand.New(rand.NewSource(100))
	t := flag.Int("t", 50, "number of transactions")
	k := flag.Int("k", 50, "soliton distribution parameter")
	flag.Parse()

	testOverlap(rng, *t, *k)
}
