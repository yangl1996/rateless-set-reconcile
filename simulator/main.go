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
	arrivalBurstSize := flag.Int("b", 1, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 50000, "decoder memory")
	detectThreshold := flag.Int("th", 5, "detector threshold")
	transactionRate := flag.Float64("txgen", 5, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 100*time.Second, "simulation duration")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	reportInterval := flag.Duration("r", 1*time.Second, "tracing interval")
	mainSeed := flag.Int64("seed", 1, "randomness seed")
	warmupDuration := flag.Duration("w", 20*time.Second, "warmup duration")
	numNodes := flag.Int("n", 20, "number of nodes in the simulation")
	averageDegree := flag.Int("d", 8, "average network degree")
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
	topo := loadCitiesTopology()
	servers := newServers(s, *numNodes, *mainSeed, config)
	for _, s := range servers {
		topo.register(s)
	}
	s.SetTopology(topo)
	connected := make(map[struct{from, to int}]struct{})
	for i := 0; i < (*numNodes)*(*averageDegree)/2; i++ {
		for {
			from := mainRNG.Intn(*numNodes)
			to := mainRNG.Intn(*numNodes)
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
	servers[0].latencySketch = newTransactionLatencySketch(*warmupDuration)
	fmt.Println("# node 0 peers", len(servers[0].handlers))

	receivedCodewordRate := difference[int]{}
	for cur := time.Duration(0); cur < *simDuration; cur += *reportInterval {
		s.RunUntil(cur)
		receivedCodewordRate.record(servers[0].receivedCodewords)
		if cur < *warmupDuration {
			fmt.Print("# ")
		}
		fmt.Println(s.Time().Seconds(), float64(receivedCodewordRate.get()) / (*reportInterval).Seconds())
	}

	d := servers[0].decodedTransactions
	r := servers[0].receivedCodewords
	fmt.Println("# received rate transaction", float64(d)/s.Time().Seconds())
	fmt.Println("# overhead", float64(r)/float64(d))
	qts := servers[0].latencySketch.getQuantiles([]float64{0.05, 0.50, 0.95})
	fmt.Println("# latency p5", qts[0], "p50", qts[1], "p95", qts[2])
}
