package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"math/rand"
	"time"
)

type serverMetric struct {
	decodedTransactions int
	receivedTransactions int
	duplicateTransactions int
	receivedCodewords    int
}

func (s *serverMetric) resetMetric() {
	s.decodedTransactions = 0
	s.receivedTransactions = 0
	s.duplicateTransactions = 0
	s.receivedCodewords = 0
}

type serverConfig struct {
	blockArrivalIntv float64
	blockArrivalBurst int

	senderConfig
}

type handler struct {
	*sender
	*receiver
}

type peer struct {
	*handler
	delay time.Duration
}

type server struct {
	handlers map[des.Module]peer
	peers []des.Module
	rng *rand.Rand

	serverConfig

	latencySketch *distributionSketch
	serverMetric

	received map[uint64]struct{}
}

var handlerSeed int64 = 0

func (a *server) newHandler(disableSender bool) *handler {
	handlerSeed += 1
	return &handler{
		sender: &sender{
			Encoder: &riblt.Encoder[transaction]{},
			senderConfig: a.senderConfig,
			sendWindow: 1,	// otherwise tryFillSendWindow always returns
			disabled: disableSender,
			shardIndex: rand.New(rand.NewSource(handlerSeed)),
		},
		receiver: &receiver{
			Decoder: &riblt.Decoder[transaction]{},
		},
	}
}

func connectServers(a, b *server, delay time.Duration) {
	a.handlers[b] = peer{a.newHandler(false), delay}
	a.peers = append(a.peers, b)
	b.handlers[a] = peer{b.newHandler(true), delay}
	b.peers = append(b.peers, a)
}

func newServers(simulator *des.Simulator, n int, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server {
			handlers: make(map[des.Module]peer),
			serverConfig: config,
			rng: rand.New(rand.NewSource(int64(i))),
			received: make(map[uint64]struct{}),
		}
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		simulator.ScheduleMessage(des.OutgoingMessage{newBa, nil, intv}, s)
		res = append(res, s)
	}
	return res
}

func (s *server) collectOutgoingMessages(outbox []des.OutgoingMessage) []des.OutgoingMessage {
	for _, peer := range s.peers {
		handler := s.handlers[peer]
		for _, msg := range handler.sender.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, handler.delay})
		}
		handler.sender.outbox = handler.sender.outbox[:0]
		for _, msg := range handler.receiver.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, handler.delay})
		}
		handler.receiver.outbox = handler.receiver.outbox[:0]
	}
	return outbox
}

func (s *server) forwardTransaction(tx riblt.HashedSymbol[transaction], exclude des.Module) {
	for _, peer := range s.peers {
		handler := s.handlers[peer]
		if peer != exclude {
			handler.sender.onTransaction(tx)
			handler.receiver.onTransaction(tx)
		}
	}
}

func (s *server) HandleMessage(payload any, from des.Module, timestamp time.Duration) []des.OutgoingMessage {
	var outbox []des.OutgoingMessage
	if ba, isBa := payload.(blockArrival); isBa {
		for i := 0; i < ba.n; i++ {
			tx := txgen.generate(timestamp)
			s.forwardTransaction(riblt.HashedSymbol[transaction]{tx, tx.Hash()}, nil)
			s.received[tx.idx] = struct{}{}
			s.receivedTransactions += 1
		}
		// schedule itself the next block arrival
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		outbox = append(outbox, des.OutgoingMessage{newBa, nil, intv})
	} else {
		n := s.handlers[from]
		switch m := payload.(type) {
		case codeword:
			remote, decoded := n.onCodeword(m)
			if decoded {
				for _, tx := range remote {
					if _, there := s.received[tx.Symbol.idx]; !there {
						s.latencySketch.recordTxLatency(tx.Symbol, timestamp)
						s.forwardTransaction(tx, from)
						s.received[tx.Symbol.idx] = struct{}{}
						s.decodedTransactions += 1
						s.receivedTransactions += 1
					} else {
						s.duplicateTransactions += 1
					}
				}
			}
			s.receivedCodewords += 1
		case ack:
			remote := n.onAck(m)
			for _, tx := range remote {
				s.receivedCodewords += 1
				if _, there := s.received[tx.Symbol.idx]; !there {
					s.latencySketch.recordTxLatency(tx.Symbol, timestamp)
					s.forwardTransaction(tx, from)
					s.received[tx.Symbol.idx] = struct{}{}
					s.decodedTransactions += 1
					s.receivedTransactions += 1
				} else {
					s.duplicateTransactions += 1
				}
			}
		default:
			panic("unknown message type")
		}
	}
	outbox = s.collectOutgoingMessages(outbox)
	return outbox
}

