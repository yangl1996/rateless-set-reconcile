package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
)

type codeword struct {
	riblt.CodedSymbol[transaction]
	newBlock bool
	threshold uint64
	salt uint64
}

type ack struct {
	ackBlock bool
	ackStart bool
	txs []riblt.HashedSymbol[transaction]
}

type blockArrival struct {
	n int
}

