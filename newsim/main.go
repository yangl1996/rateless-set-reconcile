package main

import (
	"flag"
	"fmt"
	"github.com/aclements/go-moremath/stats"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"log"
	"math/rand"
	"os"
	"sort"
	"time"
)

var txgen = &transactionGenerator{}

var RNG *rand.Rand

var L = log.New(os.Stderr, "", 0)

func main() {
	arrivalBurstSize := flag.Int("b", 1, "transaction arrival burst size")
	transactionRate := flag.Float64("txgen", 5, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 100*time.Second, "simulation duration")
	warmupDuration := flag.Duration("w", 20*time.Second, "warm-up duration")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	topologyFile := flag.String("topo", "", "topology file")
	numShards := flag.Int("s", 64, "number of shards to use")
	algorithm := flag.String("a", "coding", "algorithm to use, options are coding and pull")
	flag.Parse()

	RNG = rand.New(rand.NewSource(1))

	serverConfig := serverConfig{
		// Rate parameter for the block arrival interval distribution.
		// Transactions arrive in bursts to simulate the burstiness in decoding
		// (of transactions from other, unsimulated peers).
		blockArrivalIntv:  *transactionRate / float64(*arrivalBurstSize) / float64(time.Second),
		blockArrivalBurst: *arrivalBurstSize,
	}
	senderConfig := senderConfig{
		controlOverhead: *controlOverhead,
		numShards:       *numShards,
	}

	topo, N := loadTopology(*topologyFile)
	s := &des.Simulator{}
	servers := newServers(s, N, serverConfig)
	for _, s := range servers {
		s.latencySketch = newDistributionSketch(*warmupDuration)
	}
	for _, conn := range topo {
		switch *algorithm {
		case "coding":
			connectCodingServers(servers[conn.a], servers[conn.b], conn.delay, senderConfig)
		case"pull":
			connectPullServers(servers[conn.a], servers[conn.b], conn.delay)
		}
	}
	fmt.Println("#", N, "nodes, node 0 num peers", len(servers[0].handlers))

	warmed := false
	numEvents := 0
	lastSimTime := time.Duration(0)
	lastRealTime := time.Now()
	reportInterval := time.Duration(1) * time.Second
	for cur := time.Duration(0); cur < *simDuration; cur += reportInterval {
		s.RunUntil(cur)
		if cur > *warmupDuration {
			if warmed == false {
				warmed = true
				for _, s := range servers {
					s.resetMetric()
				}
			}
		}

		L.Printf("%.2fs %d queued %.2f ev/s sim %.2f ev/s real %.2fx speed up\n", s.Time().Seconds(), s.EventsQueued(), float64(s.EventsDelivered()-numEvents)/(s.Time()-lastSimTime).Seconds(), float64(s.EventsDelivered()-numEvents)/time.Now().Sub(lastRealTime).Seconds(), (s.Time()-lastSimTime).Seconds()/time.Now().Sub(lastRealTime).Seconds())
		numEvents = s.EventsDelivered()
		lastSimTime = s.Time()
		lastRealTime = time.Now()
	}

	fmt.Println("# moments: mean, stddev, p5, p25, p50, p75, p95")
	fmt.Println("# received transaction rate", collectMoments(servers, func(srv *server) float64 {
		return float64(srv.receivedTransactions) / (s.Time() - *warmupDuration).Seconds()
	}))
	fmt.Println("# duplicate transaction rate", collectMoments(servers, func(srv *server) float64 {
		return float64(srv.duplicateTransactions) / (s.Time() - *warmupDuration).Seconds()
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
