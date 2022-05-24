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
}

func testController(K int, s, rinit float64, timeout int, loss float64, drop float64) float64 {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(K), 0.03, 0.5)

	d := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}}
	dvirt := &decoder{ldpc.NewDecoder(experiments.TestKey, 262144), []receivedCodeword{}}
	e := ldpc.NewEncoder(experiments.TestKey, dist, K)

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
		stub, _ := d.AddCodeword(cw)
		d.rxWindow = append(d.rxWindow, receivedCodeword{stub, step})
		return
	}

	r := rinit
	c := 0.0
	l := 0
	cw := 0

	cvirt := 0.0

	totCw := 0
	totLoss := 0
	for {
		step += 1
		if (rand.Float64() < s) {
			tx := experiments.RandomTransaction()
			e.AddTransaction(tx)
		}

		c += r
		for c >= 1.0 {
			codeword := e.ProduceCodeword()
			c -= 1.0
			cw += 1
			if step > 100000 {
				totCw += 1
			}

			add(d, codeword)

			cvirt += drop
			if cvirt < 1.0 {
				add(dvirt, codeword)
				r -= (loss*0.1/1000.0)
			} else {
				cvirt -= 1.0
			}
		}
		loss := scan(dvirt)
		r += (0.1/1000.0)*float64(loss)
		realLoss := scan(d)
		l += realLoss
		if step > 100000 {
			totLoss += realLoss
		}
		if step%1000 == 0 {
			fmt.Println(step, r, float64(l)/float64(cw))
			l = 0
			cw = 0
		}
		if step > 500000 {
			break
		}
	}
	return float64(totLoss) / float64(totCw)
}

func main() {
	gamma := flag.Float64("g", 0.02, "target loss rate")
	drop := flag.Float64("d", 0.066, "period of artificial drop")
	flag.Parse()
	fmt.Println("# ms rate  loss")
	lossRate := testController(50, 0.6, 0.1, 500, *gamma, *drop)
	fmt.Printf("# loss rate: %.5f\n", lossRate)
}
