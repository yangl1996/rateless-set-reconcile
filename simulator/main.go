package main

import (
	"flag"
	//"github.com/DataDog/sketches-go/ddsketch"
	"github.com/yangl1996/rateless-set-reconcile/des"
	"time"
)


var txgen = newTransactionGenerator()

func main() {
	arrivalBurstSize := flag.Int("b", 500, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 1000*time.Second, "simulation duration")
	networkDelay := flag.Duration("d", 100*time.Millisecond, "network one-way propagation time")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)")
	flag.Parse()

	config := nodeConfig{
		detectThreshold: *detectThreshold,
		controlOverhead: *controlOverhead,
	}

	s := &des.Simulator{}
	s.SetDefaultDelay(*networkDelay)
	servers := newServers(s, 2, 1, serverConfig{
		// Rate parameter for the block arrival interval distribution.
		// Transactions arrive in bursts to simulate the burstiness in decoding
		// (of transactions from other, unsimulated peers).
		blockArrivalIntv: *transactionRate / float64(*arrivalBurstSize) / float64(time.Second),
		blockArrivalBurst: *arrivalBurstSize,
		nodeConfig: config,
		decoderMemory: *decoderMem,
	})
	connectServers(servers[0], servers[1])

	s.RunUntil(*simDuration)
}
