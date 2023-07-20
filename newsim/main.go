package main

import (
	"flag"
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"time"
	"github.com/aclements/go-moremath/stats"
	"sort"
)

var txgen = &transactionGenerator{}

func main() {
	arrivalBurstSize := flag.Int("b", 1, "transaction arrival burst size")
	transactionRate := flag.Float64("txgen", 5, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 100*time.Second, "simulation duration")
	warmupDuration := flag.Duration("w", 20*time.Second, "warmup duration")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	reportInterval := flag.Duration("r", 1*time.Second, "tracing interval")
	topologyFile := flag.String("topo", "", "topology file")
	flag.Parse()

	config := serverConfig {
		// Rate parameter for the block arrival interval distribution.
		// Transactions arrive in bursts to simulate the burstiness in decoding
		// (of transactions from other, unsimulated peers).
		blockArrivalIntv: *transactionRate / float64(*arrivalBurstSize) / float64(time.Second),
		blockArrivalBurst: *arrivalBurstSize,
		senderConfig: senderConfig{
			controlOverhead: *controlOverhead,
		},
	}

	topo, N := loadTopology(*topologyFile)
	s := &des.Simulator{}
	servers := newServers(s, N, config)
	for _, s := range servers {
		s.latencySketch = newDistributionSketch(*warmupDuration)
	}
	for _, conn := range topo {
		connectServers(servers[conn.a], servers[conn.b], conn.delay)
	}
	fmt.Println("# node 0 num peers", len(servers[0].handlers))

	receivedCodewordRate := difference[int]{}
	warmed := false
	for cur := time.Duration(0); cur < *simDuration; cur += *reportInterval {
		s.RunUntil(cur)
		if cur > *warmupDuration {
			if warmed == false {
				warmed = true
				for _, s := range servers {
					s.resetMetric()
				}
				receivedCodewordRate.record(servers[0].receivedCodewords)
			} else {
				receivedCodewordRate.record(servers[0].receivedCodewords)
				fmt.Println(s.Time().Seconds(), float64(receivedCodewordRate.get()) / (*reportInterval).Seconds())
			}
		}
	}

	fmt.Println("# moments: mean, stddev, p5, p25, p50, p75, p95")
	fmt.Println("# received transaction rate", collectMoments(servers, func(srv *server) float64 {
		return float64(srv.receivedTransactions) / (s.Time() - *warmupDuration).Seconds()
	}))
	fmt.Println("# overhead", collectMoments(servers, func(s *server) float64 {
		return float64(s.receivedCodewords) / float64(s.decodedTransactions)
	}))
	fmt.Println("# latency p5", collectMoments(servers, func(s *server) float64 {
		return s.latencySketch.getQuantiles([]float64{0.05})[0]
	}))
	fmt.Println("# latency p50", collectMoments(servers, func(s *server) float64 {
		return s.latencySketch.getQuantiles([]float64{0.50})[0]
	}))
	fmt.Println("# latency p95", collectMoments(servers, func(s *server) float64 {
		return s.latencySketch.getQuantiles([]float64{0.95})[0]
	}))
}

func collectMoments(servers []*server, metric func(s *server) float64) []float64 {
	res := []float64{}
	s := stats.Sample{}
	for _, server := range servers {
		s.Xs = append(s.Xs, metric(server))
	}
	sort.Float64s(s.Xs)
	s.Sorted = true

	res = append(res, s.Mean())
	res = append(res, s.StdDev())
	res = append(res, s.Quantile(0.05))
	res = append(res, s.Quantile(0.25))
	res = append(res, s.Quantile(0.50))
	res = append(res, s.Quantile(0.75))
	res = append(res, s.Quantile(0.95))
	return res
}
