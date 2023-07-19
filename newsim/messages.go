package main

import (
	"github.com/yangl1996/rateless-set-reconcile/riblt"
)

type codeword struct {
	riblt.CodedSymbol[transaction]
	newBlock bool
}

type ack struct {
	ackBlock bool
}

type blockArrival struct {
	n int
}

