package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
)

type message interface {
	size() int
}

type codeword struct {
	riblt.CodedSymbol[transaction]
	newBlock  bool
	startHash uint64
	endHash   uint64
}

var TXSIZE int

func (c codeword) size() int {
	return 8 + TXSIZE + 8 + 8
}

type ack struct {
	ackBlock bool
	ackStart bool
	txs      []riblt.HashedSymbol[transaction]
}

func (a ack) size() int {
	return len(a.txs) * TXSIZE + 8
}

type blockArrival struct {
	n int
}

func (a blockArrival) size() int {
	return 0
}

type response struct {
	payload riblt.HashedSymbol[transaction]
}

func (r response) size() int {
	return TXSIZE
}

type announce struct {
	hash uint64
}

func (a announce) size() int {
	return 32
}

type request struct {
	hash uint64
}

func (r request) size() int {
	return 32
}

type initialBroadcast struct {
	payload riblt.HashedSymbol[transaction]
}

func (i initialBroadcast) size() int {
	return TXSIZE
}
