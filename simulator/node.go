package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/yangl1996/soliton"
	"math/rand"
	"hash"
)

var testKey = [lt.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

type senderConfig struct {
	controlOverhead float64
	detectThreshold int
}

type sender struct {
	buffer       []lt.Transaction[transaction]
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

func (n *sender) onAck(ack ack) {
	n.inFlight -= 1
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	n.tryFillSendWindow()
}

func (n *sender) onTransaction(tx lt.Transaction[transaction]) {
	n.buffer = append(n.buffer, tx)
	n.tryFillSendWindow()
}

func (n *sender) tryFillSendWindow() {
	for n.inFlight < n.sendWindow {
		cw, yes := n.tryProduceCodeword()
		if !yes {
			return
		}
		n.outbox = append(n.outbox, cw)
		n.inFlight += 1
	}
	return
}

func (n *sender) tryProduceCodeword() (codeword, bool) {
	cw := codeword{}
	if !n.encodingCurrentBlock {
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

	touchedHashesSet map[uint32]struct{}
	touchedHashes []uint32
	sipHasher hash.Hash64
}

func (n *receiver) markHasSeen(tx lt.Transaction[transaction]) {
	n.sipHasher.Reset()
	n.sipHasher.Write(tx.Hash())
	hash := (uint32)(n.sipHasher.Sum64())
	if _, there := n.touchedHashesSet[hash]; !there {
		n.touchedHashesSet[hash] = struct{}{}
		n.touchedHashes = append(n.touchedHashes, hash)
		if len(n.touchedHashes) > 50000 {
			delete(n.touchedHashesSet, n.touchedHashes[0])
			n.touchedHashes = n.touchedHashes[1:50000]
		}
	}
}

func (n *receiver) hasSeen(tx lt.Transaction[transaction]) bool {
	n.sipHasher.Reset()
	n.sipHasher.Write(tx.Hash())
	hash := (uint32)(n.sipHasher.Sum64())
	_, there := n.touchedHashesSet[hash]
	return there
}

func(n *receiver) onCodeword(cw codeword) []lt.Transaction[transaction] {
	for _, hash := range cw.Members() {
		if _, there := n.touchedHashesSet[hash]; !there {
			n.touchedHashesSet[hash] = struct{}{}
			n.touchedHashes = append(n.touchedHashes, hash)
			if len(n.touchedHashes) > 50000 {
				delete(n.touchedHashesSet, n.touchedHashes[0])
				n.touchedHashes = n.touchedHashes[1:50000]
			}
		}
	}

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

