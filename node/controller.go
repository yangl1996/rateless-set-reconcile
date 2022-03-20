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

type peer struct {
	newTxToSender chan<- *ldpc.Transaction
	newTxToReceiver chan<- *ldpc.Transaction
}

func (h *peer) notifyNewTransaction(t *ldpc.Transaction) {
	h.newTxToSender <- t
	h.newTxToReceiver <- t
}

func newPeer(conn io.ReadWriter, decoded chan<- *ldpc.Transaction, importTx []*ldpc.Transaction, K, M uint64, initRate, minRate, incConstant, targetLoss float64, decodeTimeout time.Duration) *peer {
	peerLoss := make(chan int, 100)
	ourLoss := make(chan int, 100)
	senderNewTx := make(chan *ldpc.Transaction, 100)
	receiverNewTx := make(chan *ldpc.Transaction, 100)

	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), K, 0.03, 0.5)
	s := sender{
		tx:                   gob.NewEncoder(conn),
		encoder:              ldpc.NewEncoder(testKey, dist, int(K)),
		cwRate:               initRate,
		rateIncreaseConstant: incConstant,
		rateDecreaseConstant: incConstant*targetLoss,
		minRate: minRate,
		sendTimer:            time.NewTimer(time.Duration(1.0 / initRate * float64(time.Second))),
		peerLoss:             peerLoss,
		ourLoss: ourLoss,
		newTransaction:       senderNewTx,
	}

	r := receiver{
		rx:          gob.NewDecoder(conn),
		decoder:     ldpc.NewDecoder(testKey, int(M)),
		peerLoss: peerLoss,
		ourLoss: ourLoss,
		decodedTransaction: decoded,
		newTransaction: receiverNewTx,
		timeout:     decodeTimeout,
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

	K uint64
	M uint64
	initRate float64
	minRate float64
	incConstant float64
	targetLoss float64
	decodeTimeout time.Duration
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
			p := newPeer(conn, c.decodedTransaction, nil, c.K, c.M, c.initRate, c.minRate, c.incConstant, c.targetLoss, c.decodeTimeout)
			c.peers = append(c.peers, p)
		}
	}
}
