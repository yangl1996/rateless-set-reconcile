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
	detectThreshold int
	controlOverhead float64
}

type nodeMetric struct {
	receivedTransactions int
	receivedCodewords int
	sentCodewords int
	queuedTransactions int
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
	encodingCurrentBlock bool
	currentBlockReceived bool

	// outgoing msgs
	outbox []any
}

func (n *node) onTransaction(tx *ldpc.Transaction) {
	n.buffer = append(n.buffer, tx)
	n.queuedTransactions += 1
	n.tryFillSendWindow()
}

func (n *node) onAck(ack ack) {
	n.inFlight -= 1
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	blockSize := int(float64(n.sendWindow) / n.controlOverhead)
	if len(n.buffer) >= blockSize {
		n.sendWindow += 1
	} else {
		n.sendWindow -= 1
		// detectThreshold is a reasonable lower bound for sendWindow. The
		// receiver cannot do anything without that many codewords. TODO: it
		// may also make sense to send that many codewords when starting a new
		// block, regardless of window usage.
		if n.sendWindow < n.detectThreshold {
			n.sendWindow = n.detectThreshold
		}
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
		blockSize := int(float64(n.sendWindow) / n.controlOverhead)
		if len(n.buffer) >= blockSize {
			cw.newBlock = true
			n.encodingCurrentBlock = true
			dist := soliton.NewRobustSoliton(distRandSource, uint64(blockSize), 0.03, 0.5)
			n.Encoder.Reset(dist, blockSize)
			// move buffer into block
			for i := 0; i < blockSize; i++ {
				res := n.Encoder.AddTransaction(n.buffer[i])
				if !res {
					fmt.Println("Warning: duplicate transaction exists in window")
				}
			}
			n.buffer = n.buffer[blockSize:]
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
	// TODO: add new transactions to the queue?
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

var distRandSource = rand.New(rand.NewSource(1))

func newNode(config nodeConfig, decoderMemory int) *node {
	n := &node{
		Encoder: ldpc.NewEncoder(experiments.TestKey, nil, 0),
		Decoder: ldpc.NewDecoder(experiments.TestKey, decoderMemory),
		nodeConfig: config,
		sendWindow: config.detectThreshold,
	}
	return n
}


func main() {
	arrivalBurstSize := flag.Int("b", 500, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 1000 * time.Second, "simulation duration")
	networkDelay := flag.Duration("d", 100 * time.Millisecond, "network one-way propagation time")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)") 
	flag.Parse()

	config := nodeConfig{
		detectThreshold: *detectThreshold,
		controlOverhead: *controlOverhead,
	}

	nodes := []*node{newNode(config, *decoderMem), newNode(config, *decoderMem)}
	s := &simulator{}

	// Rate parameter for the block arrival interval distribution. Transactions
	// arrive in bursts to simulate the burstiness in decoding (of transactions
	// from other, unsimulated peers).
	meanIntv := *transactionRate / float64(*arrivalBurstSize) / float64(time.Second)
	getIntv := func() time.Duration {
		return time.Duration(rand.ExpFloat64() / meanIntv)
	}
	// schedule the arrival of first transactions
	s.queueMessage(getIntv(), 0, blockArrival{})
	s.queueMessage(getIntv(), 1, blockArrival{})
	// main simulation loop
	lastReport := time.Duration(0)
	lastCodewordCount := 0
	fmt.Println("# time(s)    codeword rate       queue      window")
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
			for i := 0; i < *arrivalBurstSize; i++ {
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
		// report metrics
		for s.time - lastReport >= time.Second {
			lastReport += time.Second
			fmt.Println(lastReport.Seconds(), nodes[0].sentCodewords-lastCodewordCount, len(nodes[0].buffer), nodes[0].sendWindow)
			lastCodewordCount = nodes[0].sentCodewords
		}
	}
	durs := s.time.Seconds()
	fmt.Printf("# received rate tx=%.2f, cw=%.2f, overhead=%.2f, generate rate tx=%.2f\n", float64(nodes[0].receivedTransactions)/durs, float64(nodes[0].receivedCodewords)/durs, float64(nodes[0].receivedCodewords)/float64(nodes[0].receivedTransactions), float64(nodes[0].queuedTransactions)/durs)
}
