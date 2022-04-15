package main

import (
	"encoding/gob"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"time"
	"log"
	"github.com/DataDog/sketches-go/ddsketch"
)

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime time.Time
}

type receiver struct {
	peerId string
	rx          *gob.Decoder
	decoder                *ldpc.Decoder
	peerLoss    chan<- int
	ourLoss     chan<- int
	decodedTransaction chan<- *ldpc.Transaction
	newTransaction <-chan *ldpc.Transaction

	rxWindow               []receivedCodeword
	timeout                time.Duration
	delaySketch *ddsketch.DDSketchWithExactSummaryStatistics
}

func (r *receiver) receive(cw chan<- Codeword) error {
	for {
		newcw := Codeword{}
		err := r.rx.Decode(&newcw)
		if err != nil {
			return err
		}
		cw <- newcw
	}
}

func (r *receiver) decode(cwChan <-chan Codeword) error {
	ticker := time.NewTicker(1 * time.Second)
	cwcnt := 0
	for {
		select {
		case cw := <-cwChan:
			cwcnt += 1
			now := time.Now()
			// clean up the pending codewords
			loss := 0
			for len(r.rxWindow) > 0 {
				head := r.rxWindow[0]
				dur := now.Sub(head.receivedTime)
				if dur < r.timeout {
					break
				}
				if !head.Decoded() {
					loss += 1
				}
				head.Free()
				r.rxWindow = r.rxWindow[1:]
			}
			r.ourLoss <- loss
			// report the loss of the peer
			if cw.Loss > 0 {
				r.peerLoss <- cw.Loss
			}
			// record the codeword transmission delay
			delayms := float64(time.Now().UnixMicro() - cw.UnixMicro) / 1000.0
			r.delaySketch.Add(delayms)
			// try to decode the new codeword
			stub, buf := r.decoder.AddCodeword(cw.Codeword)
			r.rxWindow = append(r.rxWindow, receivedCodeword{stub, now})
			for _, ntx := range buf {
				r.decodedTransaction <- ntx
			}
		case tx := <-r.newTransaction:
			buf := r.decoder.AddTransaction(tx)
			for _, ntx := range buf {
				r.decodedTransaction <- ntx
			}
		case <-ticker.C:
			qts, err := r.delaySketch.GetValuesAtQuantiles([]float64{0.50, 0.95})
			if err != nil {
				qts = []float64{-1, -1}
			}
			cnt := r.delaySketch.GetCount()
			sum := r.delaySketch.GetSum()
			log.Printf("peer %s received cws %d last second delay ms median %.1f p95 %.1f mean %.1f\n", r.peerId, cwcnt, qts[0], qts[1], sum/cnt)
			r.delaySketch.Clear()
		}
	}
}
