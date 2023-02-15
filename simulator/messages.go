package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type codeword struct {
    *ldpc.Codeword
    newBlock bool
}

type ack struct {
    ackBlock bool
}

type blockArrival struct {}
