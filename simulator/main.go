package main

import (
	"flag"
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"time"
	"github.com/aclements/go-moremath/stats"
	"sort"
	"os"
)

var txgen = newTransactionGenerator()

func main() {
	arrivalBurstSize := flag.Int("b", 1, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 50000, "decoder memory")
	detectThreshold := flag.Int("th", 5, "detector threshold")
	transactionRate := flag.Float64("txgen", 5, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 100*time.Second, "simulation duration")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	reportInterval := flag.Duration("r", 1*time.Second, "tracing interval")
	mainSeed := flag.Int64("seed", 1, "randomness seed")
	warmupDuration := flag.Duration("w", 20*time.Second, "warmup duration")
	logPrefix := flag.String("prefix", "exp", "prefix of log files")
	smoothingRate := flag.Float64("smooth", 100000, "smoothing rate target")
	synchronizationPeriod := flag.Duration("sync", 0, "synchronize block generation with given period")
	targetCodewordLoss := flag.Float64("l", 0.0, "target codeword loss rate for controller")
	topologyFile := flag.String("topo", "", "topology file")
	flag.Parse()

	config := serverConfig {
		// Rate parameter for the block arrival interval distribution.
		// Transactions arrive in bursts to simulate the burstiness in decoding
		// (of transactions from other, unsimulated peers).
		blockArrivalIntv: *transactionRate / float64(*arrivalBurstSize) / float64(time.Second),
		blockArrivalBurst: *arrivalBurstSize,
		decoderMemory: *decoderMem,
		senderConfig: senderConfig{
			detectThreshold: *detectThreshold,
			controlOverhead: *controlOverhead,
		},
		receiverConfig: receiverConfig{
			detectThreshold: *detectThreshold,
			targetCodewordLoss: *targetCodewordLoss,
		},
		forceSynchronize: *synchronizationPeriod,
	}

	topo, N := loadTopology(*topologyFile)
	s := &des.Simulator{}
	servers := newServers(s, N, *mainSeed, config)
	for _, s := range servers {
		s.latencySketch = newDistributionSketch(*warmupDuration)
		s.overlapSketch = newDistributionSketch(*warmupDuration)
		s.forwardRateLimiter.minInterval = time.Duration(int(1.0 / (*smoothingRate) * 1000000000))
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
	fmt.Println("# decoded transaction rate", collectMoments(servers, func(srv *server) float64 {
		return float64(srv.decodedTransactions) / (s.Time() - *warmupDuration).Seconds()
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
	qts := []float64{}
	for i := 0.0; i < 1.0; i += 0.01 {
		qts = append(qts, i)
	}
	for i, s := range servers {
		qtres := s.overlapSketch.getQuantiles(qts)
		filename := fmt.Sprintf("%s-overlap-%d.csv", *logPrefix, i)
		file, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		for idx, val := range qtres {
			fmt.Fprintln(file, float64(idx) * 0.01, val)
		}
	}
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
