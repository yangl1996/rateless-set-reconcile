package main

import (
	"github.com/yangl1996/rateless-set-reconcile/des"
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"math/rand"
	"time"
)

type serverMetric struct {
	decodedTransactions   int
	receivedTransactions  int
	duplicateTransactions int
	receivedBytes int
}

func (s *serverMetric) resetMetric() {
	s.decodedTransactions = 0
	s.receivedTransactions = 0
	s.duplicateTransactions = 0
	s.receivedBytes = 0
}

type serverConfig struct {
	blockArrivalIntv  float64
	blockArrivalBurst int
	initialFlood bool
}

type algorithm interface {
	collectOutgoingMessages(peer des.Module, delay time.Duration, outbox []des.OutgoingMessage) []des.OutgoingMessage
	forwardTransaction(tx riblt.HashedSymbol[transaction])
	handleMessage(msg any) []riblt.HashedSymbol[transaction]
}

type peer struct {
	algorithm
	delay time.Duration
}

type server struct {
	handlers map[des.Module]peer
	peers    []des.Module
	rng      *rand.Rand

	serverConfig

	latencySketch *distributionSketch
	serverMetric

	received map[uint64]struct{}
}

func newServers(simulator *des.Simulator, n int, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server{
			handlers:     make(map[des.Module]peer),
			serverConfig: config,
			rng:          rand.New(rand.NewSource(int64(i))),
			received:     make(map[uint64]struct{}),
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
		outbox = handler.collectOutgoingMessages(peer, handler.delay, outbox)
	}
	return outbox
}

func (s *server) forwardTransaction(tx riblt.HashedSymbol[transaction], exclude des.Module) {
	for _, peer := range s.peers {
		handler := s.handlers[peer]
		if peer != exclude {
			handler.forwardTransaction(tx)
		}
	}
}

func (s *server) HandleMessage(payload any, from des.Module, timestamp time.Duration) []des.OutgoingMessage {
	var outbox []des.OutgoingMessage
	if ba, isBa := payload.(blockArrival); isBa {
		for i := 0; i < ba.n; i++ {
			tx := txgen.generate(timestamp)
			if s.initialFlood {
				hashed := riblt.HashedSymbol[transaction]{tx, tx.Hash()}
				for _, peer := range s.peers {
					outbox = append(outbox, des.OutgoingMessage{initialBroadcast{hashed}, peer, s.handlers[peer].delay})
				}
			} else {
				s.forwardTransaction(riblt.HashedSymbol[transaction]{tx, tx.Hash()}, nil)
			}
			s.received[tx.idx] = struct{}{}
			s.receivedTransactions += 1
		}
		// schedule itself the next block arrival
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		outbox = append(outbox, des.OutgoingMessage{newBa, nil, intv})
	} else {
		if ib, isIb := payload.(initialBroadcast); isIb {
			tx := ib.payload
			if _, there := s.received[tx.Symbol.idx]; !there {
				s.latencySketch.recordTxLatency(tx.Symbol, timestamp)
				s.forwardTransaction(tx, from)
				s.received[tx.Symbol.idx] = struct{}{}
				s.decodedTransactions += 1
				s.receivedTransactions += 1
			} else {
				panic("receiving duplicate tx in initial broadcast")
			}
		} else {
			n := s.handlers[from]
			decoded := n.handleMessage(payload)
			for _, tx := range decoded {
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
		s.receivedBytes += payload.(message).size()
	}
	outbox = s.collectOutgoingMessages(outbox)
	return outbox
}
