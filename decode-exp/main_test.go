package main

import (
	"testing"
	"strconv"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

func BenchmarkExperiment(b *testing.B) {
	numTxs := []int{2000, 5000, 10000, 20000, 50000}
	for _, nt := range numTxs {
		name := strconv.Itoa(nt)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(ldpc.TxSize * nt * 2))
			for i := 0; i < b.N; i++ {
				err := runExperiment(0, 0, 0, 1000, nt, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", 42)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
