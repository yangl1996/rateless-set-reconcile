package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
)

type senderConfig struct {
	controlOverhead float64
	numShards int
}

type sender struct {
	buffer       []riblt.HashedSymbol[transaction]
	*riblt.Encoder[transaction]

	// send window
	sendWindow int
	inFlight   int
	encodingCurrentBlock bool
	currentBlockAckCount int

	// outgoing msgs
	outbox []any

	disabled bool

	senderConfig
	shardSchedule []int
	nextShard int
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
			shardSize := ((1<<64) - 1) / uint64(len(n.shardSchedule))
			cw.startHash = uint64(n.shardSchedule[n.nextShard]) * shardSize 
			cw.endHash = uint64((n.shardSchedule[n.nextShard] + 1) % len(n.shardSchedule)) * shardSize
			n.nextShard = (n.nextShard + 1) % len(n.shardSchedule)
			n.encodingCurrentBlock = true
			n.currentBlockAckCount = 0
			n.sendWindow = 1
			n.inFlight = 0
			n.Encoder.Reset()
			// move buffer into block
			tidx := 0
			for tidx < len(n.buffer) {
				v := n.buffer[tidx]
				if (cw.startHash < cw.endHash && v.Hash >= cw.startHash && v.Hash < cw.endHash) || (cw.startHash >= cw.endHash && (v.Hash >= cw.startHash || v.Hash < cw.endHash)) {
					n.Encoder.AddHashedSymbol(v)
					l := len(n.buffer)-1
					n.buffer[tidx] = n.buffer[l]
					n.buffer = n.buffer[:l]
				} else {
					tidx += 1
				}
			}
		} else {
			return cw, false
		}
	}
	if n.inFlight < n.sendWindow {
		cw.CodedSymbol = n.Encoder.ProduceNextCodedSymbol()
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

func (n *receiver) onCodeword(cw codeword) ([]riblt.HashedSymbol[transaction], bool) {
	if n.currentBlockReceived && !cw.newBlock {
		return nil, false
	}
	ack := ack{}
	if cw.newBlock {
		ack.ackStart = true
		n.currentBlockReceived = false
		n.currentBlockCount = 0
		tidx := 0
		for tidx < len(n.buffer) {
			v := n.buffer[tidx]
			if (cw.startHash < cw.endHash && v.Hash >= cw.startHash && v.Hash < cw.endHash) || (cw.startHash >= cw.endHash && (v.Hash >= cw.startHash || v.Hash < cw.endHash)) {
				n.Decoder.AddHashedSymbol(v)
				l := len(n.buffer)-1
				n.buffer[tidx] = n.buffer[l]
				n.buffer = n.buffer[:l]
			} else {
				tidx += 1
			}
		}
	}
	n.Decoder.AddCodedSymbol(cw.CodedSymbol)
	n.Decoder.TryDecode()
	n.currentBlockCount += 1
	if n.Decoder.Decoded()  {
		n.currentBlockReceived = true
		ack.ackBlock = true
		for _, tx := range n.Local() {
			ack.txs = append(ack.txs, tx)
		}
		n.outbox = append(n.outbox, ack)
		res := []riblt.HashedSymbol[transaction]{}
		for _, v := range n.Decoder.Remote() {
			res = append(res, v)
		}
		n.Decoder.Reset()
		return res, true
	} else {
		n.outbox = append(n.outbox, ack)
		return nil, false
	}
}

func (n *receiver) onTransaction(tx riblt.HashedSymbol[transaction]) {
	//n.Decoder.AddHashedSymbol(tx)
	n.buffer = append(n.buffer, tx)
}
