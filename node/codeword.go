package main

import (
    "github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type Codeword struct {
	*ldpc.Codeword
	Loss int
}
