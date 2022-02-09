package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"encoding/gob"
	"time"
	"io"
)

type sender struct {
	tx *gob.Encoder
	encoder *ldpc.Encoder
	txRate float64	// codeword sending rate in s^-1
	rateIncreaseConstant float64
	rateDecreaseConstant float64
	nextCodeword Codeword

	sendTimer *time.Timer
	peerLoss <-chan int
	ourLoss chan int
	shutdown chan struct{}
	newTransaction chan *ldpc.Transaction
}

func (s *sender) loop() error {
	for {
		select {
		case <-s.shutdown:
			return nil
		case l := <-s.peerLoss:
			s.txRate += s.rateIncreaseConstant * float64(l)
		case l := <-s.ourLoss:
			s.nextCodeword.Loss += l
		case tx := <-s.newTransaction:
			s.encoder.AddTransaction(tx)
		case <-s.sendTimer.C:
			// schedule the next event
			s.sendTimer.Reset(time.Duration(1.0 / s.txRate * float64(time.Second)))
			// send the codeword
			s.nextCodeword.Codeword = s.encoder.ProduceCodeword()
			err := s.tx.Encode(s.nextCodeword)
			if err != nil {
				return err
			}
			s.nextCodeword.Loss = 0
		}
	}
	panic("unreachable")
}

type indexedCodeword struct {
	peerIdx int
	*ldpc.Codeword
}

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime time.Time
}

type receiver struct {
	rx *gob.Decoder
	newCodeword chan<- indexedCodeword
	peerLoss chan<- int
	peerIdx int

	decoder *ldpc.Decoder
	rxWindow []receivedCodeword
	thirdPartyTransactions []*ldpc.Transaction
	timeout time.Duration
}

func (r *receiver) receiveCodeword() error {
	var cw Codeword
	for {
		err := r.rx.Decode(&cw)
		if err != nil {
			return err
		}
		r.newCodeword <- indexedCodeword{r.peerIdx, cw.Codeword}
		r.peerLoss <- cw.Loss
	}
}

type peer struct {
	sender
	receiver
}

type controller struct {
	newCodeword chan indexedCodeword	// should only be used for receiving
	peers []peer
	newPeer chan io.ReadWriter
}

func (c *controller) loop() error {
	for {
		select {
		case conn := <-c.newPeer:
			peerLoss := make(chan int, 100)
			r := receiver {
				rx: gob.NewDecoder(conn),
				newCodeword: c.newCodeword,
				peerLoss: peerLoss,
				peerIdx: len(c.peers),
			}
			s := sender {
				tx: gob.NewEncoder(conn),
				peerLoss: peerLoss,
				ourLoss: make(chan int, 100),
				shutdown: make(chan struct{}),
				newTransaction: make(chan *ldpc.Transaction, 100),
			}
			go r.receiveCodeword()
			go s.loop()
		case codeword := <-c.newCodeword:
			idx := codeword.peerIdx
			now := time.Now()
			// clean up the pending codewords
			loss := 0
			for len(c.peers[idx].receiver.rxWindow) > 0 {
				head := c.peers[idx].receiver.rxWindow[0]
				dur := now.Sub(head.receivedTime)
				if dur < c.peers[idx].receiver.timeout {
					break
				}
				if !head.Decoded() {
					loss += 1
				}
				head.Free()
				c.peers[idx].receiver.rxWindow = c.peers[idx].receiver.rxWindow[1:]
			}
			c.peers[idx].sender.ourLoss <- loss
			// insert the new codeword
			lastDecoded := idx
			stub, newTx := c.peers[idx].receiver.decoder.AddCodeword(codeword.Codeword)
			c.peers[idx].receiver.rxWindow = append(c.peers[idx].receiver.rxWindow, receivedCodeword{stub, now})
			for len(newTx) != 0 {
				// forward the new transactions from the last-decoded node to all peers
				for idx := range c.peers {
					if idx != lastDecoded {
						c.peers[idx].receiver.thirdPartyTransactions = append(c.peers[idx].receiver.thirdPartyTransactions, newTx...)
					}
					for _, ntx := range newTx {
						c.peers[idx].sender.newTransaction <- ntx
					}
				}
				newTx = nil
				for idx := range c.peers {
					if len(c.peers[idx].receiver.thirdPartyTransactions) != 0 {
						for _, ntx := range c.peers[idx].receiver.thirdPartyTransactions {
							buffer := c.peers[idx].receiver.decoder.AddTransaction(ntx)
							newTx = append(newTx, buffer...)
						}
						break
					}
				}
			}
		}
	}
}
