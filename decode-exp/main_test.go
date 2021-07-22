package main

import (
	"testing"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

func BenchmarkExperiment(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(ldpc.TxSize)
	err := runExperiment(0, 0, 0, 1000, b.N, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", 42)
	if err != nil {
		b.Fatal(err)
	}
}
