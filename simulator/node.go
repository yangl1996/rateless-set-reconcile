package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"github.com/yangl1996/soliton"
	"math/rand"
	"time"
)

var testKey = [lt.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
var distRandSource = rand.New(rand.NewSource(1))

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

type nodeConfig struct {
	detectThreshold int
	controlOverhead float64
}

type nodeMetric struct {
	decodedTransactions int
	receivedCodewords    int
	sentCodewords        int
	queuedTransactions   int
}

type node struct {
	*lt.Encoder[transaction]
	*lt.Decoder[transaction]
	curCodewords []*lt.PendingCodeword[transaction]
	buffer       []lt.Transaction[transaction]

	nodeConfig
	nodeMetric

	// send window
	sendWindow int
	inFlight   int

	// send/receive acking
	encodingCurrentBlock bool
	currentBlockReceived bool

	// outgoing msgs
	rng *rand.Rand
	outbox []any
}

func (n *node) onTransaction(tx lt.Transaction[transaction]) []lt.Transaction[transaction] {
	n.buffer = append(n.buffer, tx)
	n.queuedTransactions += 1
	n.tryFillSendWindow()
	return n.Decoder.AddTransaction(tx)
}

func (n *node) onAck(ack ack) {
	n.inFlight -= 1
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	n.tryFillSendWindow()
}

func (n *node) tryFillSendWindow() {
	for n.inFlight < n.sendWindow {
		cw, yes := n.tryProduceCodeword()
		if !yes {
			return
		}
		n.outbox = append(n.outbox, cw)
		n.sentCodewords += 1
		n.inFlight += 1
	}
	return
}

func (n *node) tryProduceCodeword() (codeword, bool) {
	cw := codeword{}
	if !n.encodingCurrentBlock {
		minBlockSize := int(float64(n.detectThreshold) / n.controlOverhead)
		if len(n.buffer) >= minBlockSize {
			// move to the next block
			blockSize := len(n.buffer)
			n.sendWindow = int(float64(blockSize) * n.controlOverhead)
			cw.newBlock = true
			n.encodingCurrentBlock = true
			dist := soliton.NewRobustSoliton(n.rng, uint64(blockSize), 0.03, 0.5)
			n.Encoder.Reset(dist, blockSize)
			// move buffer into block
			for i := 0; i < blockSize; i++ {
				res := n.Encoder.AddTransaction(n.buffer[i])
				if !res {
					fmt.Println("Warning: duplicate transaction exists in window")
				}
			}
			n.buffer = n.buffer[blockSize:]
			// We could send detectThreshold codewords when starting the
			// new block, regardless of window usage, because the receiver
			// cannot do anything (acking the block, for one and foremost)
			// before receiving that many codewords. However, sending too
			// much may lead to queuing inside the network.
		} else {
			return cw, false
		}
	}
	cw.Codeword = n.Encoder.ProduceCodeword()
	return cw, true
}

func (n *node) onCodeword(cw codeword) []lt.Transaction[transaction] {
	n.receivedCodewords += 1
	if cw.newBlock {
		n.curCodewords = n.curCodewords[:0]
		n.currentBlockReceived = false
	}
	stub, tx := n.Decoder.AddCodeword(cw.Codeword)
	// TODO: add newly received transactions to the sending queue?
	n.decodedTransactions += len(tx)
	n.curCodewords = append(n.curCodewords, stub)
	var res []lt.Transaction[transaction]
	for _, v := range tx {
		res = append(res, v)
	}

	if !n.currentBlockReceived && len(n.curCodewords) > n.detectThreshold {
		decoded := true
		for _, c := range n.curCodewords {
			if !c.Decoded() {
				decoded = false
				break
			}
		}
		if decoded {
			n.currentBlockReceived = true
			n.outbox = append(n.outbox, ack{true})
			return res
		}
	}
	n.outbox = append(n.outbox, ack{false})
	return res
}

func newNode(rng *rand.Rand, config nodeConfig, decoderMemory int) *node {
	n := &node{
		Encoder:    lt.NewEncoder[transaction](rng, testKey, nil, 0),
		Decoder:    lt.NewDecoder[transaction](testKey, decoderMemory),
		nodeConfig: config,
		// TODO: we would like to be able to leave sendWindow as zero during
		// init, but we adjust send window when creating a new block (setting
		// it to blockSize*controlOverhead). We only try creating a new block
		// when there is enough space in the window, so there is a chicken and
		// egg problem. Setting it to detectThreshold is a good adhoc fix
		// though, since it does not alter the behavior of the system (sending
		// will only start when the first block is filled, whose size is not
		// controlled by sendWindow anyway), and sendWindow will never go below
		// detectThreshold.
		sendWindow: config.detectThreshold,
		rng: rng,
	}
	return n
}
