package main

import (
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"math/rand"
	"time"
)

type handler struct {
	*node
	ingressBuffer []lt.Transaction[transaction]
}

type server struct {
	handlers map[des.Module]*handler

	rng *rand.Rand
	serverConfig

	latencySketch *transactionLatencySketch
}

type serverConfig struct {
	blockArrivalIntv float64
	blockArrivalBurst int

	nodeConfig
	decoderMemory int
}

func connectServers(a, b *server) {
	ha := &handler{
		node: newNode(a.rng, a.nodeConfig, a.decoderMemory),
	}
	hb := &handler{
		node: newNode(b.rng, b.nodeConfig, b.decoderMemory),
	}
	a.handlers[b] = ha
	b.handlers[a] = hb
}

func newServers(simulator *des.Simulator, n int, startingSeed int64, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server {
			handlers: make(map[des.Module]*handler),
			rng: rand.New(rand.NewSource(startingSeed+int64(i))),
			serverConfig: config,
		}
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		simulator.ScheduleMessage(des.OutgoingMessage{newBa, nil, intv}, s)
		res = append(res, s)
	}
	return res
}

func (s *server) collectOutgoingMessages() []des.OutgoingMessage {
	outbox := []des.OutgoingMessage{}
	for peer, handler := range s.handlers {
		for _, msg := range handler.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, time.Duration(0)})
		}
		handler.outbox = handler.outbox[:0]
	}
	return outbox
}

func (s *server) forwardTransactions(from *handler, txs []lt.Transaction[transaction]) {
	for _, handler := range s.handlers {
		if from != handler {
			handler.ingressBuffer = append(handler.ingressBuffer, txs...)
		}
	}
}

func (s *server) HandleMessage(payload any, from des.Module, timestamp time.Duration) []des.OutgoingMessage {
	if ba, isBa := payload.(blockArrival); isBa {
		for i := 0; i < ba.n; i++ {
			tx := txgen.generate(timestamp)
			for _, handler := range s.handlers {
				// freshly generated transactions, guaranteed not to cause any decoding, so we can be sure
				// the returned decoded transactions is empty
				handler.onTransaction(tx)
			}
		}
		outbox := s.collectOutgoingMessages()
		// schedule itself the next block arrival
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		outbox = append(outbox, des.OutgoingMessage{newBa, nil, intv})
		return outbox
	} else {
		n := s.handlers[from]
		switch m := payload.(type) {
		case codeword:
			buf := n.onCodeword(m)
			for _, val := range buf {
				s.latencySketch.record(val.Data(), timestamp)
			}
			s.forwardTransactions(n, buf)
			// forward the decoded transactions to others,
			// handling the chain reaction (this is so painful)
			for {
				acted := false
				for _, handler := range(s.handlers) {
					for _, t := range handler.ingressBuffer {
						acted = true
						buf = handler.onTransaction(t)
						for _, val := range buf {
							s.latencySketch.record(val.Data(), timestamp)
						}
						s.forwardTransactions(handler, buf)
					}
					handler.ingressBuffer = handler.ingressBuffer[:0]
				}
				if !acted {
					break
				}
			}
		case ack:
			n.onAck(m)
		default:
			panic("unknown message type")
		}
		return s.collectOutgoingMessages()
	}
}

