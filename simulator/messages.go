package main

import (
	"github.com/yangl1996/rateless-set-reconcile/lt"
)

type codeword struct {
    lt.Codeword[transaction]
    newBlock bool
}

type ack struct {
    ackBlock bool
}

type blockArrival struct {}
