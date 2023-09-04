package main

import (
	"github.com/yangl1996/rateless-set-reconcile/des"
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"time"
)

func connectPullServers(a, b *server, delay time.Duration) {
	a.handlers[b] = peer{&pull{
		known: make(map[uint64]riblt.HashedSymbol[transaction]),
	}, delay}
	a.peers = append(a.peers, b)
	b.handlers[a] = peer{&pull{
		known: make(map[uint64]riblt.HashedSymbol[transaction]),
	}, delay}
	b.peers = append(b.peers, a)
}

type pull struct {
	known  map[uint64]riblt.HashedSymbol[transaction]
	outbox []any
}

func (c *pull) collectOutgoingMessages(peer des.Module, delay time.Duration, outbox []des.OutgoingMessage) []des.OutgoingMessage {
	for _, msg := range c.outbox {
		outbox = append(outbox, des.OutgoingMessage{msg, peer, delay})
		c.outbox = c.outbox[:0]
	}
	return outbox
}

func (c *pull) forwardTransaction(tx riblt.HashedSymbol[transaction]) {
	c.known[tx.Hash] = tx
	c.outbox = append(c.outbox, announce{tx.Hash})
}

func (c *pull) handleMessage(msg any) (int, []riblt.HashedSymbol[transaction]) {
	switch m := msg.(type) {
	case announce:
		if _, there := c.known[m.hash]; !there {
			c.outbox = append(c.outbox, request{m.hash})
		}
		return 0, nil
	case request:
		c.outbox = append(c.outbox, response{c.known[m.hash]})
		return 0, nil
	case response:
		c.known[m.payload.Hash] = m.payload
		return 1, []riblt.HashedSymbol[transaction]{m.payload}
	default:
		panic("unknown message type")
	}
}
