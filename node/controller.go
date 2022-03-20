package main

import (
	"encoding/gob"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"io"
	"log"
	"math/rand"
	"time"
)

type sender struct {
	tx                   *gob.Encoder
	encoder              *ldpc.Encoder
	cwRate               float64 // codeword sending rate in s^-1
	rateIncreaseConstant float64
	rateDecreaseConstant float64
	minRate				float64
	nextCodeword         Codeword

	sendTimer      *time.Timer
	peerLoss       <-chan int
	ourLoss        <-chan int
	newTransaction <-chan *ldpc.Transaction
}

func (s *sender) loop() error {
	ticker := time.NewTicker(1 * time.Second)
	log.Println("sender started")
	for {
		select {
		case l := <-s.peerLoss:
			s.cwRate += s.rateIncreaseConstant * float64(l)
		case l := <-s.ourLoss:
			s.nextCodeword.Loss += l
		case tx := <-s.newTransaction:
			s.encoder.AddTransaction(tx)
		case <-s.sendTimer.C:
			// schedule the next event
			s.cwRate -= s.rateDecreaseConstant
			if s.cwRate < s.minRate {
				s.cwRate = s.minRate
			}
			s.sendTimer.Reset(time.Duration(1.0 / s.cwRate * float64(time.Second)))
			// send the codeword
			s.nextCodeword.Codeword = s.encoder.ProduceCodeword()
			err := s.tx.Encode(s.nextCodeword)
			if err != nil {
				return err
			}
			s.nextCodeword.Loss = 0
		case <-ticker.C:
			log.Println("codeword rate", s.cwRate)
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
		}
	}
}

type peer struct {
	newTxToSender chan<- *ldpc.Transaction
	newTxToReceiver chan<- *ldpc.Transaction
}

func (h *peer) notifyNewTransaction(t *ldpc.Transaction) {
	h.newTxToSender <- t
	h.newTxToReceiver <- t
}

func newPeer(conn io.ReadWriter, decoded chan<- *ldpc.Transaction, importTx []*ldpc.Transaction) *peer {
	peerLoss := make(chan int, 100)
	ourLoss := make(chan int, 100)
	senderNewTx := make(chan *ldpc.Transaction, 100)
	receiverNewTx := make(chan *ldpc.Transaction, 100)

	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	s := sender{
		tx:                   gob.NewEncoder(conn),
		encoder:              ldpc.NewEncoder(testKey, dist, 50),
		cwRate:               1.0,
		rateIncreaseConstant: 0.1,
		rateDecreaseConstant: 0.002,
		minRate: 1.0,
		sendTimer:            time.NewTimer(time.Duration(1.0 / 1.0 * float64(time.Second))),
		peerLoss:             peerLoss,
		ourLoss: ourLoss,
		newTransaction:       senderNewTx,
	}

	r := receiver{
		rx:          gob.NewDecoder(conn),
		decoder:     ldpc.NewDecoder(testKey),
		peerLoss: peerLoss,
		ourLoss: ourLoss,
		decodedTransaction: decoded,
		newTransaction: receiverNewTx,
		timeout:     time.Duration(0.5 * float64(time.Second)),
	}
	for _, existingTx := range importTx {
		r.decoder.AddTransaction(existingTx)
	}

	go func() {
		err := s.loop()
		if err != nil {
			panic(err)
		}
	}()
	cwCh := make(chan *ldpc.Codeword, 1000)
	go func() {
		err := r.receive(cwCh)
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := r.decode(cwCh)
		if err != nil {
			panic(err)
		}
	}()
	return &peer{senderNewTx, receiverNewTx}
}

type controller struct {
	peers            []*peer
	newPeerConn          chan io.ReadWriter
	decodedTransaction chan *ldpc.Transaction
	localTransaction chan *ldpc.Transaction
}

var testKey = [ldpc.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func (c *controller) loop() error {
	log.Println("controller started")
	for {
		select {
		case tx := <-c.localTransaction:
			for _, peer := range c.peers {
				peer.notifyNewTransaction(tx)
			}
		case tx := <-c.decodedTransaction:
			for _, peer := range c.peers {
				peer.notifyNewTransaction(tx)
			}
		case conn := <-c.newPeerConn:
			log.Println("new peer")
			p := newPeer(conn, c.decodedTransaction, nil)
			c.peers = append(c.peers, p)
		}
	}
}
