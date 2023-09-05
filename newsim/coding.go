package main

import (
	"github.com/yangl1996/rateless-set-reconcile/des"
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"time"
)

func connectCodingServers(a, b *server, delay time.Duration, config senderConfig) {
	randomizer := RNG.Uint64()
	a.handlers[b] = peer{coding{
		sender: &sender{
			Encoder:       &riblt.Encoder[transaction]{},
			senderConfig:  config,
			sendWindow:    1, // otherwise tryFillSendWindow always returns
			shardSchedule: RNG.Perm(config.numShards),
			shardRandomizer: randomizer,
		},
		receiver: nil,
	}, delay}
	a.peers = append(a.peers, b)
	b.handlers[a] = peer{coding{
		sender: nil,
		receiver: &receiver{
			Decoder: &riblt.Decoder[transaction]{},
			shardRandomizer: randomizer,
		},
	}, delay}
	b.peers = append(b.peers, a)
}

type coding struct {
	*sender
	*receiver
}

func (c coding) collectOutgoingMessages(peer des.Module, delay time.Duration, outbox []des.OutgoingMessage) []des.OutgoingMessage {
	if c.sender != nil {
		for _, msg := range c.sender.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, delay})
		}
		c.sender.outbox = c.sender.outbox[:0]
	}
	if c.receiver != nil {
		for _, msg := range c.receiver.outbox {
			outbox = append(outbox, des.OutgoingMessage{msg, peer, delay})
		}
		c.receiver.outbox = c.receiver.outbox[:0]
	}
	return outbox
}

func (c coding) forwardTransaction(tx riblt.HashedSymbol[transaction]) {
	c.sender.onTransaction(tx)
	c.receiver.onTransaction(tx)
}

func (c coding) handleMessage(msg any) []riblt.HashedSymbol[transaction] {
	switch m := msg.(type) {
	case codeword:
		remote, decoded := c.onCodeword(m)
		if decoded {
			return remote
		} else {
			return nil
		}
	case ack:
		decoded := c.onAck(m)
		return decoded
	default:
		panic("unknown message type")
	}
}

type senderConfig struct {
	controlOverhead float64
	numShards       int
}

type sender struct {
	buffer []riblt.HashedSymbol[transaction]
	*riblt.Encoder[transaction]

	// send window
	sendWindow            float64
	inFlight              int
	encodingCurrentBlock  bool
	currentBlockAckCount  int
	receivingCurrentBlock bool

	// outgoing msgs
	outbox []any

	senderConfig
	shardSchedule []int
	shardRandomizer uint64
	nextShard     int
}

func (n *sender) onAck(ack ack) []riblt.HashedSymbol[transaction] {
	if n == nil {
		return nil
	}
	if ack.ackBlock {
		n.encodingCurrentBlock = false
		n.receivingCurrentBlock = false
	}
	if ack.ackStart {
		n.currentBlockAckCount = 0
		n.receivingCurrentBlock = true
	}
	if n.receivingCurrentBlock {
		n.currentBlockAckCount += 1
		n.inFlight -= 1
		n.sendWindow += n.controlOverhead
		if n.sendWindow < 1 {
			n.sendWindow = 1
		}
	}
	//n.sendWindow = float64(n.currentBlockAckCount) * n.controlOverhead
	n.tryFillSendWindow()
	return ack.txs
}

func (n *sender) onTransaction(tx riblt.HashedSymbol[transaction]) {
	if n == nil {
		return
	}
	n.buffer = append(n.buffer, tx)
	n.tryFillSendWindow()
}

func (n *sender) tryFillSendWindow() {
	if n == nil {
		return
	}
	for {
		cw, yes, burst := n.tryProduceCodeword()
		if !yes {
			return
		}
		n.outbox = append(n.outbox, cw)
		n.inFlight += 1
		for i := 0; i < burst; i++ {
			cw := codeword{}
			cw.CodedSymbol = n.Encoder.ProduceNextCodedSymbol()
			n.outbox = append(n.outbox, cw)
			n.inFlight += 1
		}
	}
	return
}

func (n *sender) tryProduceCodeword() (codeword, bool, int) {
	if n == nil {
		return codeword{}, false, 0
	}
	cw := codeword{}
	burstSize := 0
	if !n.encodingCurrentBlock {
		// NOTE: here we start the next block (shard) no matter whether there is content for that shard
		// on sender's side. This is because the receiver might have content, which can then be transmitted
		// to the sender. This opportunity is lost when the sender chooses not to initiate reconciliation.
		//if len(n.buffer) > 0 {
		// move to the next block
		cw.newBlock = true
		shardSize := ((1 << 64) - 1) / uint64(len(n.shardSchedule))
		cw.startHash = uint64(n.shardSchedule[n.nextShard]) * shardSize
		cw.endHash = uint64((n.shardSchedule[n.nextShard]+1)%len(n.shardSchedule)) * shardSize
		// move buffer into block
		//	okay := false
		n.Encoder.Reset()
		tidx := 0
		for tidx < len(n.buffer) {
			v := n.buffer[tidx]
			shardHash := v.Hash * n.shardRandomizer
			if (cw.startHash < cw.endHash && shardHash >= cw.startHash && shardHash < cw.endHash) || (cw.startHash >= cw.endHash && (shardHash >= cw.startHash || shardHash < cw.endHash)) {
				n.Encoder.AddHashedSymbol(v)
				l := len(n.buffer) - 1
				n.buffer[tidx] = n.buffer[l]
				n.buffer = n.buffer[:l]
				//			okay = true
			} else {
				tidx += 1
			}
		}
		//	if !okay {
		//		return cw, false
		//	}
		burstSize = n.currentBlockAckCount * 2 / 3
		n.nextShard = (n.nextShard + 1) % len(n.shardSchedule)
		n.encodingCurrentBlock = true
		n.sendWindow = float64(n.currentBlockAckCount) * n.controlOverhead
		if n.sendWindow < 1 {
			n.sendWindow = 1
		}
		n.currentBlockAckCount = 0
		n.inFlight = 0
		//} else {
		//	return cw, false
		//}
	}
	// NOTE: there is a big difference whether to turn inFlight into float
	// or turn sendWindow to int. The former allows the second coded symbol
	// to be sent much earlier than would be in the latter scheme.
	if float64(n.inFlight) < n.sendWindow {
		cw.CodedSymbol = n.Encoder.ProduceNextCodedSymbol()
		return cw, true, burstSize
	} else {
		return cw, false, burstSize
	}
}

type receiver struct {
	*riblt.Decoder[transaction]
	buffer []riblt.HashedSymbol[transaction]

	currentBlockReceived bool
	currentBlockSize     int
	currentBlockCount    int
	shardRandomizer uint64

	// outgoing msgs
	outbox []any
}

func (n *receiver) onCodeword(cw codeword) ([]riblt.HashedSymbol[transaction], bool) {
	if n == nil {
		return nil, false
	}
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
			shardHash := v.Hash * n.shardRandomizer
			if (cw.startHash < cw.endHash && shardHash >= cw.startHash && shardHash < cw.endHash) || (cw.startHash >= cw.endHash && (shardHash >= cw.startHash || shardHash < cw.endHash)) {
				n.Decoder.AddHashedSymbol(v)
				l := len(n.buffer) - 1
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
	if n.Decoder.Decoded() {
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
	if n == nil {
		return
	}
	//n.Decoder.AddHashedSymbol(tx)
	n.buffer = append(n.buffer, tx)
}
