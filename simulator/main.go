package main

import (
	"flag"
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"time"
	"math/rand"
)


var txgen = newTransactionGenerator()

func main() {
	arrivalBurstSize := flag.Int("b", 500, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 50000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 1000*time.Second, "simulation duration")
	networkDelay := flag.Duration("d", 100*time.Millisecond, "network one-way propagation time")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	reportInterval := flag.Duration("r", 1*time.Second, "tracing interval")
	mainSeed := flag.Int64("seed", 1, "randomness seed")
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
		},
	}

	mainRNG := rand.New(rand.NewSource(*mainSeed))
	s := &des.Simulator{}
	s.SetDefaultDelay(*networkDelay)
	servers := newServers(s, 100, *mainSeed, config)
	connected := make(map[struct{from, to int}]struct{})
	for i := 0; i < 100*8; i++ {
		for {
			from := mainRNG.Intn(100)
			to := mainRNG.Intn(100)
			if from == to {
				continue
			}
			pair1 := struct{from, to int}{from, to}
			pair2 := struct{from, to int}{to, from}
			if _, there := connected[pair1]; there {
				continue
			}
			if _, there := connected[pair2]; there {
				continue
			}
			connected[pair1] = struct{}{}
			connectServers(servers[from], servers[to])
			break
		}
	}
	servers[0].latencySketch = newTransactionLatencySketch()
	fmt.Println("# node 0 peers", len(servers[0].handlers))

	receivedCodewordRate := difference[int]{}
	for cur := time.Duration(0); cur < *simDuration; cur += *reportInterval {
		s.RunUntil(cur)
		r := 0
		for _, h := range servers[0].handlers {
			r += h.receivedCodewords
		}
		receivedCodewordRate.record(r)
		fmt.Println(s.Time().Seconds(), float64(receivedCodewordRate.get()) / (*reportInterval).Seconds())
	}

	d := 0
	r := 0
	for _, h := range servers[0].handlers {
		d += h.decodedTransactions
		r += h.receivedCodewords
	}
	fmt.Println("# received rate transaction", float64(d)/s.Time().Seconds())
	fmt.Println("# overhead", float64(r)/float64(d))
	qts := servers[0].latencySketch.getQuantiles([]float64{0.05, 0.50, 0.95})
	fmt.Println("# latency p5", qts[0], "p50", qts[1], "p95", qts[2])
}
