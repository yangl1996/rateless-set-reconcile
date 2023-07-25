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
	currentBlockAckCount int

	// outgoing msgs
	outbox []any

	disabled bool

	senderConfig
}

func (n *sender) onAck(ack ack) []riblt.HashedSymbol[transaction] {
	if ack.ackBlock {
		n.encodingCurrentBlock = false
	}
	if ack.ackStart {
		n.currentBlockAckCount = 0
	}
	n.currentBlockAckCount += 1
	n.inFlight -= 1
	n.sendWindow = int(float64(n.currentBlockAckCount) * n.controlOverhead)
	if n.sendWindow < 1 {
		n.sendWindow = 1
	}
	n.tryFillSendWindow()
	return ack.txs
}

func (n *sender) onTransaction(tx riblt.HashedSymbol[transaction]) {
	n.buffer = append(n.buffer, tx)
	n.tryFillSendWindow()
}

func (n *sender) tryFillSendWindow() {
	for {
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
	if n.disabled {
		return codeword{}, false
	}
	cw := codeword{}
	if (!n.encodingCurrentBlock) {
		if len(n.buffer) > 0 {
			// move to the next block
			cw.newBlock = true
			n.encodingCurrentBlock = true
			n.currentBlockAckCount = 0
			n.sendWindow = 1
			n.inFlight = 0
			n.Encoder.Reset()
			n.deg.Reset()
			// move buffer into block
			for _, v := range n.buffer {
				n.Encoder.AddHashedSymbol(v)
			}
			n.buffer = n.buffer[:0]
		} else {
			return cw, false
		}
	}
	if n.inFlight < n.sendWindow {
		salt := n.salt.Uint64()
		threshold := n.deg.NextThreshold()
		cw.CodedSymbol = n.Encoder.ProduceCodedSymbol(salt, threshold)
		cw.salt = salt
		cw.threshold = threshold
		return cw, true
	} else {
		return cw, false
	}
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
	if n.currentBlockReceived && !cw.newBlock {
		return false
	}
	ack := ack{}
	if cw.newBlock {
		ack.ackStart = true
		n.currentBlockReceived = false
		//n.currentBlockSize = int(cw.Count()) + len(n.buffer)
		n.currentBlockCount = 0
		for _, tx := range n.buffer {
			n.Decoder.AddHashedSymbol(tx)
		}
		n.buffer = n.buffer[:0]
	}
	n.Decoder.AddCodedSymbol(cw.CodedSymbol, cw.salt, cw.threshold)
	n.Decoder.TryDecode()
	n.currentBlockCount += 1
	if n.Decoder.Decoded()  {
		n.currentBlockReceived = true
		ack.ackBlock = true
		for _, tx := range n.Local() {
			ack.txs = append(ack.txs, tx)
		}
		n.outbox = append(n.outbox, ack)
		n.Decoder.Reset()
		return true
	} else {
		n.outbox = append(n.outbox, ack)
		return false
	}
}

func (n *receiver) onTransaction(tx riblt.HashedSymbol[transaction]) {
	n.Decoder.AddHashedSymbol(tx)
	//n.buffer = append(n.buffer, tx)
}
