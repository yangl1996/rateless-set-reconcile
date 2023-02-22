package main

import (
	"fmt"
	"math/rand"
	"flag"
	"time"
	"github.com/DataDog/sketches-go/ddsketch"
)

func main() {
	arrivalBurstSize := flag.Int("b", 500, "transaction arrival burst size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Duration("dur", 1000 * time.Second, "simulation duration")
	networkDelay := flag.Duration("d", 100 * time.Millisecond, "network one-way propagation time")
	controlOverhead := flag.Float64("c", 0.10, "control overhead (ratio between the max number of codewords sent after a block is decoded and the block size)") 
	flag.Parse()

	config := nodeConfig{
		detectThreshold: *detectThreshold,
		controlOverhead: *controlOverhead,
	}

	nodes := []*node{newNode(config, *decoderMem), newNode(config, *decoderMem)}
	s := &simulator{}
	txgen := newTransactionGenerator()

	// Rate parameter for the block arrival interval distribution. Transactions
	// arrive in bursts to simulate the burstiness in decoding (of transactions
	// from other, unsimulated peers).
	meanIntv := *transactionRate / float64(*arrivalBurstSize) / float64(time.Second)
	getIntv := func() time.Duration {
		return time.Duration(rand.ExpFloat64() / meanIntv)
	}
	// schedule the arrival of first transactions (transactions flow from 0 to 1)
	s.queueMessage(getIntv(), 0, blockArrival{})
	// main simulation loop
	lastReport := time.Duration(0)
	lastCodewordCount := 0
	latencySketch, err := ddsketch.NewDefaultDDSketch(0.01)
	if err != nil {
		panic(err)
	}
	fmt.Println("# time(s)    codeword rate       queue      window")
	for s.time <= *simDuration {
		// deliver message
		if s.drained() {
			break
		}
		dest, msg := s.nextMessage()
		switch m := msg.(type) {
		case codeword:
			txs := nodes[dest].onCodeword(m)
			for _, v := range txs {
				latency := (s.time - txgen.timestamp(v.Data())).Seconds()
				latencySketch.Add(latency)
			}
		case ack:
			nodes[dest].onAck(m)
		case blockArrival:
			for i := 0; i < *arrivalBurstSize; i++ {
				tx := txgen.generate(s.time)
				nodes[dest].onTransaction(tx)
			}
			s.queueMessage(getIntv(), dest, blockArrival{})
		default:
			panic("unknown message type")
		}
		// deliver message
		for _, v := range nodes[dest].outbox {
			s.queueMessage(*networkDelay, 1-dest, v)
		}
		nodes[dest].outbox = nodes[dest].outbox[:0]
		// report metrics
		for s.time - lastReport >= time.Second {
			lastReport += time.Second
			fmt.Println(lastReport.Seconds(), nodes[0].sentCodewords-lastCodewordCount, len(nodes[0].buffer), nodes[0].sendWindow)
			lastCodewordCount = nodes[0].sentCodewords
		}
	}
	durs := s.time.Seconds()
	fmt.Printf("# received rate tx=%.2f, cw=%.2f, overhead=%.2f\n", float64(nodes[0].receivedTransactions)/durs, float64(nodes[0].receivedCodewords)/durs, float64(nodes[0].receivedCodewords)/float64(nodes[0].receivedTransactions))
	fmt.Printf("# generate rate tx=%.2f\n", float64(nodes[0].queuedTransactions)/durs)
	qt, err := latencySketch.GetValuesAtQuantiles([]float64{0.05, 0.5, 0.95})
	if err != nil {
		panic(err)
	}
	fmt.Printf("# latency seconds p5=%.3f, p50=%.3f, p95=%.3f\n", qt[0], qt[1], qt[2])
}
