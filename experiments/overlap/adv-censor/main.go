package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
)

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime int
}

type decoder struct {
	*ldpc.Decoder
	rxWindow []receivedCodeword
}

func testController(K int, overlap, rinit float64, timeout int) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	d := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}}
	e := ldpc.NewEncoder(experiments.TestKey, dist, K)
	txset := make(map[ldpc.Transaction]struct{})

	// one step is 1ms
	step := 0
	scan := func(d *decoder) int {
		loss := 0
		for len(d.rxWindow) > 0 {
			head := d.rxWindow[0]
			dur := step - head.receivedTime
			if dur < timeout {
				break
			}
			if !head.Decoded() {
				loss += 1
			}
			head.Free()
			d.rxWindow = d.rxWindow[1:]
		}
		return loss
	}
	add := func(d *decoder, cw *ldpc.Codeword) {
		stub, txs := d.AddCodeword(cw)
		d.rxWindow = append(d.rxWindow, receivedCodeword{stub, step})
		for _, t := range txs {
			delete(txset, *t)
		}
		return
	}
	addTx := func(d *decoder, tx *ldpc.Transaction) {
		txs := d.AddTransaction(tx)
		for _, t := range txs {
			delete(txset, *t)
		}
		return
	}

	r := rinit
	c := 0.0
	start  := 1000000
	end := 2000000
	added := 0
	for {
		step += 1
		tx := experiments.RandomTransaction()
		if (rand.Float64() < overlap) {
			e.AddTransaction(tx)
			addTx(d, tx)
		} else {
			e.AddTransaction(tx)
			if step > start {
				txset[*tx] = struct{}{}
				added += 1
			}
		}
		c += r
		for c >= 1.0 {
			cw := e.ProduceCodeword()
			r -= (0.002/1000.0)
			add(d, cw)
			c -= 1.0
		}
		loss := scan(d)
		r += (0.1/1000.0)*float64(loss)
		if step > end {
			break
		}
	}
	fmt.Println(overlap, float64(added-len(txset))/float64(added), r)
	return
}

func main() {
	fmt.Println("# frac uncensored   frac received among censored    codeword rate")
	testController(50, 0.0, 1.0, 500)
	for censored := 0.8; censored < 0.99; censored += 0.01 {
		testController(50, censored, 1-censored, 500)
	}
}
