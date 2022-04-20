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

func testController(K int, s1, s2, common, r1init, r2init float64, timeout int) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	d1 := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}}
	d2 := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}}
	e1 := ldpc.NewEncoder(experiments.TestKey, dist, K)
	e2 := ldpc.NewEncoder(experiments.TestKey, dist, K)

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
		last := d
		for len(txs) > 0 {
			newtxs := []ldpc.DecodedTransaction{}
			if last == d1 {
				last = d2
			} else {
				last = d1
			}
			for _, t := range txs {
				buf := last.AddTransaction(t.Transaction)
				newtxs = append(newtxs, buf...)
			}
			txs = newtxs
		}
		return
	}

	r1 := r1init
	r2 := r2init
	c1 := 0.0
	c2 := 0.0
	for {
		step += 1
		if (rand.Float64() < s1) {
			tx := experiments.RandomTransaction()
			e1.AddTransaction(tx)
		}
		if (rand.Float64() < s2) {
			tx := experiments.RandomTransaction()
			e2.AddTransaction(tx)
		}
		if (rand.Float64() < common) {
			tx := experiments.RandomTransaction()
			e1.AddTransaction(tx)
			e2.AddTransaction(tx)
		}
		c1 += r1
		c2 += r2
		for c1 >= 1.0 {
			c := e1.ProduceCodeword()
			r1 -= (0.002/1000.0)
			add(d1, c)
			c1 -= 1.0
		}
		for c2 >= 1.0 {
			c := e2.ProduceCodeword()
			r2 -= (0.002/1000.0)
			add(d2, c)
			c2 -= 1.0
		}
		loss1 := scan(d1)
		r1 += (0.1/1000.0)*float64(loss1)
		loss2 := scan(d2)
		r2 += (0.1/1000.0)*float64(loss2)
		if step%1000 == 0 {
			fmt.Println(step, r1, r2)
		}
	}
	return
}

func testOverlap(K, N int, s1, s2, common, threshold float64) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	// create 2N transactions
	txs := []*ldpc.Transaction{}
	{
		catchConf := ldpc.NewEncoder(experiments.TestKey, dist, N*3)
		for len(txs) < N*3 {
			tx := experiments.RandomTransaction()
			ok := catchConf.AddTransaction(tx)
			if ok {
				txs = append(txs, tx)
			}
		}
	}

	test := func(r1, r2 float64) (float64, float64) {
		d1 := ldpc.NewDecoder(experiments.TestKey, 2147483647)
		e1 := ldpc.NewEncoder(experiments.TestKey, dist, K)
		e2 := ldpc.NewEncoder(experiments.TestKey, dist, K)

		txset1 := make(map[ldpc.Transaction]struct{})
		txset2 := make(map[ldpc.Transaction]struct{})
		c1 := 0.0
		c2 := 0.0
		ptr := 0
		for i := 0; i < N; i++ {
			if (rand.Float64() < s1) {
				e1.AddTransaction(txs[ptr])
				txset1[*txs[ptr]] = struct{}{}
				ptr++
			}
			if (rand.Float64() < s2) {
				e2.AddTransaction(txs[ptr])
				txset2[*txs[ptr]] = struct{}{}
				ptr++
			}
			if (rand.Float64() < common) {
				e1.AddTransaction(txs[ptr])
				e2.AddTransaction(txs[ptr])
				txset1[*txs[ptr]] = struct{}{}
				txset2[*txs[ptr]] = struct{}{}
				ptr++
			}
			c1 += r1
			c2 += r2
			for c1 >= 1.0 {
				c := e1.ProduceCodeword()
				_, newtx := d1.AddCodeword(c)
				for _, tx := range newtx {
					delete(txset1, *tx.Transaction)
					delete(txset2, *tx.Transaction)
				}
				c1 -= 1.0
			}
			for c2 >= 1.0 {
				c := e2.ProduceCodeword()
				_, newtx := d1.AddCodeword(c)
				for _, tx := range newtx {
					delete(txset1, *tx.Transaction)
					delete(txset2, *tx.Transaction)
				}
				c2 -= 1.0
			}
		}
		deliver1 := float64(N-len(txset1)) / float64(N)
		deliver2 := float64(N-len(txset2)) / float64(N)
		return deliver1, deliver2
	}

	minRate := 0.0
	maxRate := 3.0
	for r2 := s2; r2 <= maxRate; r2 += 0.02 {
		r1 := (minRate + maxRate/2)
		lastOk := maxRate
		lastFail := minRate
		for lastOk - lastFail > 0.01 {
			d1, d2 := test(r1, r2)
			if d1 > threshold && d2 > threshold {
				lastOk = r1
				r1 = (r1 + lastFail) / 2.0
			} else {
				lastFail = r1
				r1 = (r1 + lastOk) / 2.0
			}
		}
		if lastOk != maxRate {
			fmt.Printf("%.2f %.2f\n", lastOk, r2)
		}
	}
}

func main() {
	fmt.Println("# rate1 rate2 deliver1 deliver2")
	//testOverlap(50, 10000, 0.6, 0.1, 0.4, 0.95)
	testController(50, 0.6, 0.1, 0.4, 0.2, 2.0, 500)
}
