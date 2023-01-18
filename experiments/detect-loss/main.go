package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
)

func testOverlap(rng *rand.Rand, t, k, n int) (int, int) {
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

	nd := 0
	i := 0
	for ;; {
		i += 1
		cw := e.ProduceCodeword()
		stub, newtx := d.AddCodeword(cw)
		for _, tx := range newtx {
			delete(txset, *tx.Transaction)
		}
		if stub.Decoded() {
			nd += 1
		} else {
			nd = 0
		}
		if nd >= n {
			break
		}

	}
	return len(txset), i
}

func main() {
	rng := rand.New(rand.NewSource(100))
	t := flag.Int("t", 50, "number of transactions")
	m := flag.Int("m", 0, "number of transactions to fail")
	ntest := flag.Int("ntest", 100, "number of tests to run")
	n := flag.Int("n", 100, "number of codewords decoded in a row to stop")
	k := flag.Int("k", 50, "soliton distribution parameter")
	flag.Parse()

	succ := 0
	tot := 0
	for i := 0; i < *ntest; i++ {
		fail, cws := testOverlap(rng, *t, *k, *n)
		if fail <= *m {
			succ += 1
		}
		tot += cws
	}
	fmt.Println(*n, float64(*ntest-succ) / float64(*ntest), float64(tot)/float64(*ntest))
}
