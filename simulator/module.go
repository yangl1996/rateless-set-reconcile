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
}

type handler struct {
	*sender
	*receiver
}

type server struct {
	handlers map[des.Module]*handler
	decoder *lt.Decoder[transaction]

	rng *rand.Rand
	serverConfig

	latencySketch *transactionLatencySketch
	overlapSketch *transactionLatencySketch
	serverMetric
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

func connectServers(a, b *server) {
	a.handlers[b] = a.newHandler()
	b.handlers[a] = b.newHandler()
}

func newServers(simulator *des.Simulator, n int, startingSeed int64, config serverConfig) []*server {
	res := []*server{}
	for i := 0; i < n; i++ {
		s := &server {
			handlers: make(map[des.Module]*handler),
			decoder: lt.NewDecoder[transaction](testKey, config.decoderMemory),
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
		for _, msg := range handler.sender.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, time.Duration(0)})
		}
		handler.sender.outbox = handler.sender.outbox[:0]
		for _, msg := range handler.receiver.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, time.Duration(0)})
		}
		handler.receiver.outbox = handler.receiver.outbox[:0]
	}
	return outbox
}

func (s *server) forwardTransactions(txs []lt.Transaction[transaction]) {
	// TODO: we are ignoring the possibility of codewords being decoded because of this...
	for len(txs) > 0 {
		tx := txs[0]
		for _, handler := range s.handlers {
			handler.sender.onTransaction(tx)
		}
		txs = txs[1:]
		txs = append(txs, s.decoder.AddTransaction(tx)...)
	}
}

func (s *server) HandleMessage(payload any, from des.Module, timestamp time.Duration) []des.OutgoingMessage {
	if ba, isBa := payload.(blockArrival); isBa {
		txs := []lt.Transaction[transaction]{}
		for i := 0; i < ba.n; i++ {
			tx := txgen.generate(timestamp)
			txs = append(txs, tx)
		}
		s.forwardTransactions(txs)
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
			s.decodedTransactions += len(buf)
			s.receivedCodewords += 1
			s.forwardTransactions(buf)
		case ack:
			n.onAck(m)
		default:
			panic("unknown message type")
		}
		// see if we are starting a new block, and compute overlap
		outmsgs := s.collectOutgoingMessages()
		if s.overlapSketch != nil {
			for _, msg := range outmsgs {
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
		return outmsgs
	}
}

