package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
)

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime int
}

type decoder struct {
	*ldpc.Decoder
	rxWindow []receivedCodeword
	nd int
}

func testController(K int, s, rinit float64, timeout int, alpha, loss float64) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	d := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}, 0}
	e := ldpc.NewEncoder(experiments.TestKey, dist, K)

	// one step is 1ms
	scan := func(d *decoder, step int) int {
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

	add := func(d *decoder, cw *ldpc.Codeword, step int) bool {
		stub, _ := d.AddCodeword(cw)
		if stub.Decoded() {
			d.nd += 1
		} else {
			d.nd = 0
		}
		d.rxWindow = append(d.rxWindow, receivedCodeword{stub, step})
		if d.nd >= 20 {
			d.nd = 0
			return true
		} else {
			return false
		}
	}

	r := rinit
	c := 0.0	// codeword credit
	l := 0
	cw := 0

	txQueue := []*ldpc.Transaction{}
	moveWindow := func(e *ldpc.Encoder, q []*ldpc.Transaction, step int) []*ldpc.Transaction {
		k := K
		if len(q) < k {
			k = len(q)
		}
		for i := 0; i < k; i++ {
			e.AddTransaction(q[i])
		}
		fmt.Println(step, "adding", k, "transactions to coding window, queue length", len(q)-k)
		return q[k:]
	}

	for i := 0; i < K; i++ {
		tx := experiments.RandomTransaction()
		e.AddTransaction(tx)
	}
	for step := 0; step < 500000; step += 1 {
		if (rand.Float64() < s) {
			tx := experiments.RandomTransaction()
			txQueue = append(txQueue, tx)
		}

		c += r
		for c >= 1.0 {
			codeword := e.ProduceCodeword()
			c -= 1.0
			cw += 1

			canMove := add(d, codeword, step)
			if canMove {
				txQueue = moveWindow(e, txQueue, step)
			}

			r -= (loss*alpha/1000.0)
		}
		realLoss := scan(d, step)
		r += (alpha/1000.0)*float64(realLoss)
		if step%1000 == 0 {
			fmt.Println(step, r, float64(l)/float64(cw))
			l = 0
			cw = 0
		}
	}
}

func main() {
	gamma := flag.Float64("g", 0.02, "target loss rate")
	alpha := flag.Float64("a", 0.1, "controller alpha")
	flag.Parse()
	fmt.Println("# ms rate  loss")
	testController(500, 0.6, 0.1, 5000, *alpha, *gamma)
}
