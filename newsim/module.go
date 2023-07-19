package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"math/rand"
	"time"
)

type serverMetric struct {
	decodedTransactions int
	receivedCodewords    int
}

func (s *serverMetric) resetMetric() {
	s.decodedTransactions = 0
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
	rng *rand.Rand

	serverConfig

	latencySketch *distributionSketch
	serverMetric
}

func (a *server) newHandler() *handler {
	return &handler{
		sender: &sender{
			SynchronizedEncoder: &riblt.SynchronizedEncoder[transaction]{rand.New(rand.NewSource(0)), &riblt.Encoder[transaction]{}, &degseq{}},
			senderConfig: a.senderConfig,
		},
		receiver: &receiver{
			SynchronizedDecoder: &riblt.SynchronizedDecoder[transaction]{rand.New(rand.NewSource(0)), &riblt.Decoder[transaction]{}, &degseq{}},
		},
	}
}

func connectServers(a, b *server, delay time.Duration) {
	a.handlers[b] = peer{a.newHandler(), delay}
	b.handlers[a] = peer{b.newHandler(), delay}
}

func newServers(simulator *des.Simulator, n int, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server {
			handlers: make(map[des.Module]peer),
			serverConfig: config,
			rng: rand.New(rand.NewSource(int64(i))),
		}
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		simulator.ScheduleMessage(des.OutgoingMessage{newBa, nil, intv}, s)
		res = append(res, s)
	}
	return res
}

func (s *server) collectOutgoingMessages(outbox []des.OutgoingMessage) []des.OutgoingMessage {
	for peer, handler := range s.handlers {
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
	for peer, handler := range s.handlers {
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
		}
		// schedule itself the next block arrival
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		outbox = append(outbox, des.OutgoingMessage{newBa, nil, intv})
	} else {
		n := s.handlers[from]
		switch m := payload.(type) {
		case codeword:
			decoded := n.onCodeword(m)
			if decoded {
				remote := n.Remote()
				for _, tx := range remote {
					s.latencySketch.recordTxLatency(tx.Symbol, timestamp)
					s.forwardTransaction(tx, from)
				}
				s.decodedTransactions += len(remote)
			}
			s.receivedCodewords += 1
		case ack:
			n.onAck(m)
		default:
			panic("unknown message type")
		}
	}
	outbox = s.collectOutgoingMessages(outbox)
	return outbox
}

