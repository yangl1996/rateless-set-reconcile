package main

import (
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"math/rand"
	"time"
)

type serverMetric struct {
	decodedTransactions int
	receivedCodewords    int
}

func (m *serverMetric) resetMetric() {
	m.decodedTransactions = 0
	m.receivedCodewords = 0
}

type serverConfig struct {
	blockArrivalIntv float64
	blockArrivalBurst int

	decoderMemory int

	senderConfig
	receiverConfig

	forceSynchronize time.Duration
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
	decoder *lt.Decoder[transaction]

	rng *rand.Rand
	serverConfig

	latencySketch *distributionSketch
	overlapSketch *distributionSketch
	serverMetric

	forwardRateLimiter rateLimiter
}

type rateLimiter struct {
	lastScheduled time.Duration
	minInterval time.Duration
}

func (a *server) newHandler() *handler {
	return &handler{
		sender: &sender{
			Encoder: lt.NewEncoder[transaction](a.rng, testKey, nil, 0),
			rng: a.rng,
			sendWindow: a.senderConfig.detectThreshold,
			senderConfig: a.senderConfig,
		},
		receiver: &receiver{
			Decoder: a.decoder,
			receiverConfig: a.receiverConfig,
		},
	}
}

func connectServers(a, b *server, delay time.Duration) {
	a.handlers[b] = peer{a.newHandler(), delay}
	b.handlers[a] = peer{b.newHandler(), delay}
}

func newServers(simulator *des.Simulator, n int, startingSeed int64, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server {
			handlers: make(map[des.Module]peer),
			decoder: lt.NewDecoder[transaction](testKey, config.decoderMemory),
			rng: rand.New(rand.NewSource(startingSeed+int64(i))),
			serverConfig: config,
		}
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		simulator.ScheduleMessage(des.OutgoingMessage{newBa, nil, intv}, s)
		if s.forceSynchronize != 0 {
			simulator.ScheduleMessage(des.OutgoingMessage{createNewBlock{}, nil, s.forceSynchronize}, s)
		}
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

func (s *server) scheduleForwardingTransactions(out []des.OutgoingMessage, txs []lt.Transaction[transaction], ts time.Duration) []des.OutgoingMessage {
	nextSlot := ts
	if s.forwardRateLimiter.lastScheduled + s.forwardRateLimiter.minInterval > nextSlot {
		nextSlot = s.forwardRateLimiter.lastScheduled + s.forwardRateLimiter.minInterval
	}
	for _, tx := range txs {
		out = append(out, des.OutgoingMessage{loopback{tx}, nil, nextSlot-ts})
		s.forwardRateLimiter.lastScheduled = nextSlot
		nextSlot += s.forwardRateLimiter.minInterval
	}
	return out
}

func (s *server) forwardTransaction(tx lt.Transaction[transaction]) {
	canStartNewBlock := (s.forceSynchronize == 0)
	for _, handler := range s.handlers {
		handler.sender.onTransaction(tx, canStartNewBlock)
	}
}

func (s *server) HandleMessage(payload any, from des.Module, timestamp time.Duration) []des.OutgoingMessage {
	var outbox []des.OutgoingMessage
	canStartNewBlock := (s.forceSynchronize == 0)
	if ba, isBa := payload.(blockArrival); isBa {
		txs := []lt.Transaction[transaction]{}
		for i := 0; i < ba.n; i++ {
			tx := txgen.generate(timestamp)
			txs = append(txs, tx)
			buf := s.decoder.AddTransaction(tx)
			if len(buf) != 0 {
				panic("locally generated tx leading to decode")
			}
		}
		outbox = s.scheduleForwardingTransactions(outbox, txs, timestamp)
		// schedule itself the next block arrival
		intv := time.Duration(s.rng.ExpFloat64() / s.blockArrivalIntv)
		newBa := blockArrival{s.blockArrivalBurst}
		outbox = append(outbox, des.OutgoingMessage{newBa, nil, intv})
	} else if lp, isLp := payload.(loopback); isLp {
		s.forwardTransaction(lp.tx)
	} else if _, isNb := payload.(createNewBlock); isNb {
		for _, handler := range s.handlers {
			handler.sender.tryFillSendWindow(true)
		}
		outbox = append(outbox, des.OutgoingMessage{createNewBlock{}, nil, s.forceSynchronize})
	} else {
		n := s.handlers[from]
		switch m := payload.(type) {
		case codeword:
			buf := n.onCodeword(m)
			for _, val := range buf {
				s.latencySketch.recordTxLatency(val.Data(), timestamp)
			}
			s.decodedTransactions += len(buf)
			s.receivedCodewords += 1
			outbox = s.scheduleForwardingTransactions(outbox, buf, timestamp)
		case ack:
			n.onAck(m, canStartNewBlock)
		default:
			panic("unknown message type")
		}
	}
	// see if we are starting a new block, and compute overlap
	outbox = s.collectOutgoingMessages(outbox)
	if s.overlapSketch != nil {
		for _, msg := range outbox {
			if cw, is := msg.Payload.(codeword); is {
				if cw.newBlock {
					peerServer := msg.To.(*server)
					peerHandler := peerServer.handlers[s]	// peer's handler for us
					ourHandler := s.handlers[peerServer]	// our handler for the peer
					overlap := 0
					for _, tx := range ourHandler.sender.currentBlock {
						if peerHandler.receiver.HasDecoded(tx) {
							overlap += 1
						}
					}
					ratio := float64(overlap) / float64(len(ourHandler.sender.currentBlock))
					s.overlapSketch.recordRaw(ratio, timestamp)
				}
			}
		}
	}
	return outbox
}

