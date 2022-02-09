package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"encoding/gob"
	"time"
	"io"
	"math/rand"
	"github.com/yangl1996/soliton"
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

func (p *peer) ingestTransactions() []*ldpc.Transaction {
	var newTx []*ldpc.Transaction
	for _, ntx := range p.receiver.thirdPartyTransactions {
		buffer := p.receiver.decoder.AddTransaction(ntx)
		newTx = append(newTx, buffer...)
	}
	p.receiver.thirdPartyTransactions = p.receiver.thirdPartyTransactions[:0]
	return newTx
}

type controller struct {
	newCodeword chan indexedCodeword	// should only be used for receiving
	peers []peer
	newPeer chan io.ReadWriter
	allTransactions []*ldpc.Transaction
}

func newController() *controller {
	return &controller {
		newCodeword: make(chan indexedCodeword, 1000),
		newPeer: make(chan io.ReadWriter),
	}
}

var testKey = [ldpc.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

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
				decoder: ldpc.NewDecoder(testKey),
				timeout: time.Duration(0.5 * float64(time.Second)),
			}
			dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
			s := sender {
				tx: gob.NewEncoder(conn),
				encoder: ldpc.NewEncoder(testKey, dist, 50),
				txRate: 1.0,
				rateIncreaseConstant: 0.1,
				rateDecreaseConstant: 0.002,
				sendTimer: time.NewTimer(time.Duration(1.0 / 1.0 * float64(time.Second))),
				peerLoss: peerLoss,
				ourLoss: make(chan int, 100),
				shutdown: make(chan struct{}),
				newTransaction: make(chan *ldpc.Transaction, 100),
			}
			for _, existingTx := range c.allTransactions {
				r.decoder.AddTransaction(existingTx)
			}
			c.peers = append(c.peers, peer{s, r})
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
					newTx = c.peers[idx].ingestTransactions()
					if len(newTx) > 0 {
						break
					}
				}
			}
		}
	}
}
