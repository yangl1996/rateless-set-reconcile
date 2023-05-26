package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/soliton"
	"math/rand"
)

var testKey = [lt.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

type senderConfig struct {
	controlOverhead float64
	detectThreshold int
}

type sender struct {
	buffer       []lt.Transaction[transaction]
	currentBlock []lt.Transaction[transaction]
	*lt.Encoder[transaction]
	rng *rand.Rand

	// send window
	sendWindow int
	inFlight   int
	encodingCurrentBlock bool

	// outgoing msgs
	outbox []any

	senderConfig
}

func (n *sender) onAck(ack ack, mayStartNewBlock bool) {
	n.inFlight -= 1
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	n.tryFillSendWindow(mayStartNewBlock)
}

func (n *sender) onTransaction(tx lt.Transaction[transaction], mayStartNewBlock bool) {
	n.buffer = append(n.buffer, tx)
	n.tryFillSendWindow(mayStartNewBlock)
}

func (n *sender) tryFillSendWindow(mayStartNewBlock bool) {
	for n.inFlight < n.sendWindow {
		cw, yes := n.tryProduceCodeword(mayStartNewBlock)
		if !yes {
			return
		}
		n.outbox = append(n.outbox, cw)
		n.inFlight += 1
	}
	return
}

func (n *sender) tryProduceCodeword(mayStartNewBlock bool) (codeword, bool) {
	cw := codeword{}
	if (!n.encodingCurrentBlock) && mayStartNewBlock {
		//minBlockSize := int(2 / n.controlOverhead)
		minBlockSize := int(float64(n.detectThreshold) / n.controlOverhead)
		if len(n.buffer) >= minBlockSize {
			// move to the next block
			blockSize := len(n.buffer)
			n.sendWindow = int(float64(blockSize) * n.controlOverhead)
			cw.newBlock = true
			n.encodingCurrentBlock = true
			dist := soliton.NewRobustSoliton(n.rng, uint64(blockSize), 0.03, 0.5)
			n.Encoder.Reset(dist, blockSize)
			n.currentBlock = n.currentBlock[:0]
			// move buffer into block
			for i := 0; i < blockSize; i++ {
				res := n.Encoder.AddTransaction(n.buffer[i])
				if !res {
					fmt.Println("Warning: duplicate transaction exists in window")
				} else {
					n.currentBlock = append(n.currentBlock, n.buffer[i])
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
	} else if (!n.encodingCurrentBlock) && (!mayStartNewBlock) {
		return cw, false
	}
	cw.Codeword = n.Encoder.ProduceCodeword()
	return cw, true
}

type receiverConfig struct {
	detectThreshold int
}

type receiver struct {
	*lt.Decoder[transaction]
	curCodewords []*lt.PendingCodeword[transaction]

	currentBlockReceived bool

	// outgoing msgs
	outbox []any

	receiverConfig
}

func (n *receiver) onCodeword(cw codeword) []lt.Transaction[transaction] {
	if cw.newBlock {
		n.curCodewords = n.curCodewords[:0]
		n.currentBlockReceived = false
	}
	stub, tx := n.Decoder.AddCodeword(cw.Codeword)
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
			return tx
		}
	}
	n.outbox = append(n.outbox, ack{false})
	return tx
}

