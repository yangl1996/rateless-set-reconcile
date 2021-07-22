package main

import (
	"testing"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

func BenchmarkExperiment2000(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(ldpc.TxSize * 2000)
	for i := 0; i < b.N; i++ {
		err := runExperiment(0, 0, 0, 1000, 2000, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", 42)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExperiment5000(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(ldpc.TxSize * 5000)
	for i := 0; i < b.N; i++ {
		err := runExperiment(0, 0, 0, 1000, 5000, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", 42)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExperiment20000(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(ldpc.TxSize * 20000)
	for i := 0; i < b.N; i++ {
		err := runExperiment(0, 0, 0, 1000, 20000, "p(0.7)", 0, nil, nil, nil, nil, "u(0.01)", 42)
		if err != nil {
			b.Fatal(err)
		}
	}
}
