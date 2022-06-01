package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
)

func testOverlap(rng *rand.Rand, t, k, n int) int {
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

	for i := 0; i < n; i++ {
		cw := e.ProduceCodeword()
		_, newtx := d.AddCodeword(cw)
		for _, tx := range newtx {
			delete(txset, *tx.Transaction)
		}
	}
	return len(txset)
}

func main() {
	rng := rand.New(rand.NewSource(100))
	t := flag.Int("t", 50, "number of transactions")
	m := flag.Int("m", 0, "number of transactions to fail")
	ntest := flag.Int("ntest", 100, "number of transactions to fail")
	n := flag.Int("n", 100, "number of codewords")
	k := flag.Int("k", 50, "soliton distribution parameter")
	flag.Parse()

	succ := 0
	for i := 0; i < *ntest; i++ {
		fail := testOverlap(rng, *t, *k, *n)
		if fail <= *m {
			succ += 1
		}
	}
	fmt.Printf("fail rate %.2f\n", float64(*ntest-succ) / float64(*ntest))
}
