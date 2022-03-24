package main

import (
	"log"
	"encoding/gob"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"time"
)

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime time.Time
}

type receiver struct {
	rx          *gob.Decoder
	decoder                *ldpc.Decoder
	peerLoss    chan<- int
	ourLoss     chan<- int
	decodedTransaction chan<- *ldpc.Transaction
	newTransaction <-chan *ldpc.Transaction

	rxWindow               []receivedCodeword
	timeout                time.Duration
}

func (r *receiver) receive(cw chan<- *ldpc.Codeword) error {
	for {
		newcw := &Codeword{}
		err := r.rx.Decode(newcw)
		if err != nil {
			return err
		}
		if newcw.Loss > 0 {
			r.peerLoss <- newcw.Loss
		}
		cw <- newcw.Codeword
	}
}

func (r *receiver) decode(cwChan <-chan *ldpc.Codeword) error {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case cw := <-cwChan:
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
			// try to decode the new codeword
			stub, buf := r.decoder.AddCodeword(cw)
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
			log.Println("transactions stored", r.decoder.NumTransactionsReceived())
		}
	}
}
