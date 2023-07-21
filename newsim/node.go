package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"math/rand"
)

type senderConfig struct {
	controlOverhead float64
}

type sender struct {
	buffer       []riblt.HashedSymbol[transaction]
	*riblt.Encoder[transaction]
	deg *degseq
	salt *rand.Rand

	// send window
	sendWindow int
	inFlight   int
	encodingCurrentBlock bool

	// outgoing msgs
	outbox []any

	senderConfig
}

func (n *sender) onAck(ack ack) []riblt.HashedSymbol[transaction] {
	n.inFlight -= 1
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	n.tryFillSendWindow()
	return ack.txs
}

func (n *sender) onTransaction(tx riblt.HashedSymbol[transaction]) {
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
	if (!n.encodingCurrentBlock) {
		if len(n.buffer) > 0 {
			// move to the next block
			blockSize := len(n.buffer)
			n.sendWindow = int(float64(blockSize) * n.controlOverhead)
			if n.sendWindow <= 0 {
				n.sendWindow = 1
			}
			cw.newBlock = true
			n.encodingCurrentBlock = true
			n.Encoder.Reset()
			n.deg.Reset()
			// move buffer into block
			for i := 0; i < blockSize; i++ {
				n.Encoder.AddHashedSymbol(n.buffer[i])
			}
			n.buffer = n.buffer[:0]
		} else {
			return cw, false
		}
	}
	salt := n.salt.Uint64()
	threshold := n.deg.NextThreshold()
	cw.CodedSymbol = n.Encoder.ProduceCodedSymbol(salt, threshold)
	cw.salt = salt
	cw.threshold = threshold
	return cw, true
}

type receiver struct {
	*riblt.Decoder[transaction]
	buffer       []riblt.HashedSymbol[transaction]

	currentBlockReceived bool
	currentBlockSize int
	currentBlockCount int

	// outgoing msgs
	outbox []any
}

func (n *receiver) onCodeword(cw codeword) bool {
	if cw.newBlock {
		n.Decoder.Reset()
		n.currentBlockReceived = false
		n.currentBlockSize = int(cw.Count()) + len(n.buffer)
		n.currentBlockCount = 0
		for _, tx := range n.buffer {
			n.Decoder.AddHashedSymbol(tx)
		}
		n.buffer = n.buffer[:0]
	}
	n.Decoder.AddCodedSymbol(cw.CodedSymbol, cw.salt, cw.threshold)
	n.Decoder.TryDecode()
	n.currentBlockCount += 1
	if !n.currentBlockReceived && (n.Decoder.Decoded() || n.currentBlockCount > n.currentBlockSize * 2) {
		n.currentBlockReceived = true
		ack := ack{true, nil}
		for _, tx := range n.Local() {
			ack.txs = append(ack.txs, tx)
		}
		n.outbox = append(n.outbox, ack)
		return true
	} else {
		n.outbox = append(n.outbox, ack{false, nil})
		return false
	}
}

func (n *receiver) onTransaction(tx riblt.HashedSymbol[transaction]) {
	n.buffer = append(n.buffer, tx)
}
