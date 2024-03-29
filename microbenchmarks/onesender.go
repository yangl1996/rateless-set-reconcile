package microbenchmarks

import (
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func SimulateTwoSendersOverlap(Nruns int, N int, common int, sender1cw int) int {
	resCh := make(chan int, Nruns)
	oneSimulation := func(runIdx int) {
		seed := int64(runIdx)
		dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
		dist2 := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
		e1 := lt.NewEncoder[Transaction](rand.New(rand.NewSource(seed)), TestKey, dist1, N)
		e2 := lt.NewEncoder[Transaction](rand.New(rand.NewSource(seed)), TestKey, dist2, N)
		d := lt.NewDecoder[Transaction](TestKey, 2147483647)

		var txIdx uint64
		toDecode := make(map[uint64]struct{})

		for i := 0; i < common; i++ {
			tx := GetTransaction(uint64(txIdx))
			if e1.AddTransaction(tx) {
				e2.AddTransaction(tx)
				toDecode[uint64(txIdx)] = struct{}{}
			} else {
				i -= 1
			}
			txIdx += 1
		}

		for i := common; i < N; i++ {
			tx := GetTransaction(uint64(txIdx))
			if e1.AddTransaction(tx) {
				toDecode[uint64(txIdx)] = struct{}{}
			} else {
				i -= 1
			}
			txIdx += 1
		}
		for i := common; i < N; i++ {
			tx := GetTransaction(uint64(txIdx))
			if e2.AddTransaction(tx) {
				toDecode[uint64(txIdx)] = struct{}{}
			} else {
				i -= 1
			}
			txIdx += 1
		}

		Ncw := 0
		for i := 0; i < sender1cw; i++ {
			c := e1.ProduceCodeword()
			Ncw += 1
			_, newtx := d.AddCodeword(c)
			for _, tx := range newtx {
				delete(toDecode, tx.Data().Idx)
			}
		}
		for len(toDecode) > 0 && Ncw < sender1cw + N*5 {
			c := e2.ProduceCodeword()
			Ncw += 1
			_, newtx := d.AddCodeword(c)
			for _, tx := range newtx {
				delete(toDecode, tx.Data().Idx)
			}
		}
		if len(toDecode) > 0 {
			resCh <- -1
		} else {
			resCh <- Ncw
		}
	}
	tot := 0
	for runIdx := 0; runIdx < Nruns; runIdx++ {
		go oneSimulation(runIdx)
	}
	failed := 0
	for runIdx := 0; runIdx < Nruns; runIdx++ {
		res := <-resCh
		if res == -1 {
			failed += 1
		} else {
			tot += res
		}
	}
	if Nruns > failed + 50 {
		return tot/(Nruns-failed)
	} else {
		return -1
	}
}


func SimulateOneSenderOverlap(Nruns int, N int, common int) int {
	resCh := make(chan int, Nruns)
	oneSimulation := func(runIdx int) {
		seed := int64(runIdx)
		dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(seed)), uint64(N), 0.03, 0.5)
		e := lt.NewEncoder[Transaction](rand.New(rand.NewSource(seed)), TestKey, dist, N)
		d := lt.NewDecoder[Transaction](TestKey, 2147483647)

		var txIdx uint64

		for i := 0; i < common; i++ {
			tx := GetTransaction(uint64(txIdx))
			if e.AddTransaction(tx) {
				d.AddTransaction(tx)
			} else {
				i -= 1
			}
			txIdx += 1
		}

		toDecode := make(map[uint64]struct{})
		for i := common; i < N; i++ {
			tx := GetTransaction(uint64(txIdx))
			if e.AddTransaction(tx) {
				toDecode[uint64(txIdx)] = struct{}{}
			} else {
				i -= 1
			}
			txIdx += 1
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
		resCh <- Ncw
	}
	tot := 0
	for runIdx := 0; runIdx < Nruns; runIdx++ {
		go oneSimulation(runIdx)
	}
	for runIdx := 0; runIdx < Nruns; runIdx++ {
		res := <-resCh
		tot += res
	}
	return tot/Nruns
}

