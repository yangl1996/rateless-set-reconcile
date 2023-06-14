package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/soliton"
    "math/rand"
)

func main() {
	fmt.Println("# block size 100, loss 0.5, counting threshold 5")
	fmt.Println("# overlap   cw to loss threshold     cw to finish")
	N := 100
	for overlap := 0; overlap < 100; overlap++ {
		seed := int64(overlap)
		dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
		e := lt.NewEncoder[microbenchmarks.Transaction](rand.New(rand.NewSource(seed)), microbenchmarks.TestKey, dist, N)
		d := lt.NewDecoder[microbenchmarks.Transaction](microbenchmarks.TestKey, 2147483647)

		var txIdx int

		for i := 0; i < overlap; i++ {
			tx := microbenchmarks.GetTransaction(uint64(txIdx))
			if e.AddTransaction(tx) {
				d.AddTransaction(tx)
			} else {
				i -= 1
			}
			txIdx += 1
		}

		toDecode := make(map[uint64]struct{})
		for i := overlap; i < N; i++ {
			tx := microbenchmarks.GetTransaction(uint64(txIdx))
			if e.AddTransaction(tx) {
				toDecode[uint64(txIdx)] = struct{}{}
			} else {
				i -= 1
			}
			txIdx += 1
		}

		var cwToThreshold int
		cw := 0
		stubs := []*lt.PendingCodeword[microbenchmarks.Transaction]{}
		codewords := []lt.Codeword[microbenchmarks.Transaction]{}
		for len(toDecode) > 0 {
			c := e.ProduceCodeword()
			codewords = append(codewords, c)
			cw += 1
			stub, newtx := d.AddCodeword(c)
			stubs = append(stubs, stub)
			for _, tx := range newtx {
				delete(toDecode, tx.Data().Idx)
			}
			if cwToThreshold == 0 && len(stubs) > 5 {
				decoded := 0
				for _, stub := range stubs {
					if stub.Decoded() {
						decoded += 1
					}
				}
				if decoded > int(float64(len(stubs)) * 0.5) {
					cwToThreshold = len(stubs)
				}
			}
		}
		fmt.Println(overlap, cwToThreshold, cw)
	}
}
