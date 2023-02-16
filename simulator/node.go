package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"math/rand"
)

var testKey = [ldpc.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
var distRandSource = rand.New(rand.NewSource(1))

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

func newNode(config nodeConfig, decoderMemory int) *node {
	n := &node{
		Encoder: ldpc.NewEncoder(testKey, nil, 0),
		Decoder: ldpc.NewDecoder(testKey, decoderMemory),
		nodeConfig: config,
		sendWindow: config.detectThreshold,
	}
	return n
}
