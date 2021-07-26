package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"strconv"
	"testing"
)

func BenchmarkExperiment(b *testing.B) {
	numTxs := []int{10000, 20000, 50000, 100000, 200000, 300000, 500000, 1000000}
	lookback := []uint64{2000}
	for _, nt := range numTxs {
		name := strconv.Itoa(nt)
		b.Run(name, func(b *testing.B) {
			for _, lb := range lookback {
				if lb == 0 {
					name = "Inf"
				} else {
					name = strconv.Itoa(int(lb))
				}
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(ldpc.TxSize * nt * 2))
					for i := 0; i < b.N; i++ {
						err := runExperiment(0, 0, 0, 1000, nt, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", lb, 42)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
