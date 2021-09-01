package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"strconv"
	"testing"
)

func BenchmarkExperiment(b *testing.B) {
	numTxs := []int{10000, 20000, 50000, 100000, 200000, 300000, 500000, 1000000}
	lookback := []uint64{500}
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
					cfg := ExperimentConfig {
						MirrorProb: 0,
						Seed: 42,
						TimeoutDuration: 1000,
						TimeoutCounter: nt,
						DegreeDist: "u(0.01)",
						LookbackTime: lb,
						ParallelRuns: 1,	// does not matter
						Topology: Topology {
							Servers: []Server {
								Server {
									Name: "node1",
									InitialUniqueTx: 0,
									TxArrivePattern: "p(0.7)",
								},
								Server {
									Name: "node2",
									InitialUniqueTx: 0,
									TxArrivePattern: "p(0.7)",
								},
							},
							InitialCommonTx: 0,
							Connections: []Connection {
								Connection {"node1", "node2"},
							},
						},
					}
					b.ReportAllocs()
					b.SetBytes(int64(ldpc.TxSize * nt * 2))
					for i := 0; i < b.N; i++ {
						err := runExperiment(cfg, nil, nil, nil, nil)
						if _, is := err.(TransactionCountError); !is {
							b.Fatal("decoding experiment failed")
						}
					}
				})
			}
		})
	}
}
