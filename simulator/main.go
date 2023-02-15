package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
	"time"
)

type nodeConfig struct {
	blockSize int
	detectThreshold int
}

type nodeMetric struct {
	receivedTransactions int
	receivedCodewords int
	sentCodewords int
}

type node struct {
	*ldpc.Encoder
	*ldpc.Decoder
	curCodewords []*ldpc.PendingCodeword
	buffer []*ldpc.Transaction
	
	nodeConfig
	nodeMetric

	// send window
	sendWindow int
	inFlight int

	// send/receive acking
	currentBlockDelivered bool
	currentBlockReceived bool

	// outgoing msgs
	outbox []any
}

func (n *node) onTransaction(tx *ldpc.Transaction) {
	n.buffer = append(n.buffer, tx)
	n.tryFillSendWindow()
}

func (n *node) onAck(ack ack) {
	n.inFlight -= 1
	if ack.ackBlock {
		n.currentBlockDelivered = true
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
	if n.currentBlockDelivered {
		if len(n.buffer) >= n.blockSize {
			cw.newBlock = true
			n.currentBlockDelivered = false
			// move buffer into block
			for i := 0; i < n.blockSize; i++ {
				res := n.Encoder.AddTransaction(n.buffer[i])
				if !res {
					fmt.Println("Warning: duplicate transaction exists in window")
				}
			}
			n.buffer = n.buffer[n.blockSize:]
		} else {
			return cw, false
		}
	}
	cw.Codeword = n.Encoder.ProduceCodeword()
	return cw, true
}

func (n *node) onCodeword(cw codeword) {
	n.receivedCodewords += 1
	if cw.newBlock {
		for _, c := range n.curCodewords {
			c.Free()
		}
		n.curCodewords = n.curCodewords[:0]
		n.currentBlockReceived = false
	}
	stub, tx := n.Decoder.AddCodeword(cw.Codeword)
	// TODO: add new transactions to the encoder?
	n.receivedTransactions += len(tx)
	n.curCodewords = append(n.curCodewords, stub)

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
			return
		}
	}
	n.outbox = append(n.outbox, ack{false})
	return
}


func newNode(config nodeConfig, decoderMemory int, initWindow int) *node {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(config.blockSize), 0.03, 0.5)
	n := &node{
		Encoder: ldpc.NewEncoder(experiments.TestKey, dist, config.blockSize),
		Decoder: ldpc.NewDecoder(experiments.TestKey, decoderMemory),
		nodeConfig: config,
		sendWindow: initWindow,
	}
	return n
}


func main() {
	blockSize := flag.Int("k", 500, "block size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 1000 * time.Second, "simulation duration")
	initWindow := flag.Int("initcwnd", 10, "initial codeword sending window size")
	networkDelay := flag.Duration("d", 100 * time.Millisecond, "network RTT")
	flag.Parse()

	config := nodeConfig{
		blockSize: *blockSize,
		detectThreshold: *detectThreshold,
	}

	nodes := []*node{newNode(config, *decoderMem, *initWindow), newNode(config, *decoderMem, *initWindow)}
	s := &simulator{}

	// Rate parameter for the block arrival interval distribution. Transactions
	// arrive as blocks to simulate the burstiness in decoding (of transactions
	// from other, unsimulated peers).
	meanIntv := *transactionRate / float64(*blockSize) / float64(time.Second)
	getIntv := func() time.Duration {
		return time.Duration(int(rand.ExpFloat64() / meanIntv))
	}
	// schedule the arrival of first transactions
	s.queueMessage(getIntv(), 0, blockArrival{})
	s.queueMessage(getIntv(), 1, blockArrival{})
	// main simulation loop
	for s.time <= *simDuration {
		// deliver message
		if s.drained() {
			break
		}
		dest, msg := s.nextMessage()
		switch m := msg.(type) {
		case codeword:
			nodes[dest].onCodeword(m)
		case ack:
			nodes[dest].onAck(m)
		case blockArrival:
			for i := 0; i < *blockSize; i++ {
				tx := experiments.RandomTransaction()
				nodes[dest].onTransaction(tx)
			}
			s.queueMessage(getIntv(), dest, blockArrival{})
		default:
			panic("unknown message type")
		}
		// deliver message
		for _, v := range nodes[dest].outbox {
			s.queueMessage(*networkDelay, 1-dest, v)
		}
		nodes[dest].outbox = nodes[dest].outbox[:0]
	}
}
