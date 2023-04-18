package microbenchmarks

import (
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func SimulateOneSenderOverlap(Nruns int, N int, common int) int {
	tot := 0
	for runIdx := 0; runIdx < Nruns; runIdx++ {
		seed := int64(runIdx)
		dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
		e := lt.NewEncoder[Transaction](rand.New(rand.NewSource(seed)), TestKey, dist, N)
		d := lt.NewDecoder[Transaction](TestKey, 2147483647)

		for i := 0; i < common; i++ {
			tx := GetTransaction(uint64(i))
			e.AddTransaction(tx)
			d.AddTransaction(tx)
		}
		toDecode := make(map[uint64]struct{})
		for i := common; i < N; i++ {
			tx := GetTransaction(uint64(i))
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
		tot += Ncw
	}
	return tot/Nruns
}

