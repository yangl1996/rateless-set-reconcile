package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
	"gonum.org/v1/gonum/stat/distuv"
	exprand "golang.org/x/exp/rand"
)

type codeword struct {
	*ldpc.Codeword
	newBlock bool
	ackBlock bool
}

type nodeConfig struct {
	blockSize int
	detectThreshold int
	queueDiffCoeff float64
	queueTargetCoeff float64
	targetQueueLen int
	minSendRate float64
}

type node struct {
	*ldpc.Encoder
	*ldpc.Decoder
	curCodewords []*ldpc.PendingCodeword
	buffer []*ldpc.Transaction
	
	nodeConfig

	// flags that affect the next codeword
	readySendNextBlock bool
	readyReceiveNextBlock bool
	ackedThisBlock bool

	// rate control states
	lastQueueLen int
	sendRate float64
}

func (n *node) addTransaction(tx *ldpc.Transaction) {
	n.buffer = append(n.buffer, tx)
	return
}

func (n *node) addCodeword(cw codeword) []*ldpc.Transaction {
	if cw.newBlock {
		for _, c := range n.curCodewords {
			c.Free()
		}
		n.curCodewords = n.curCodewords[:0]
		n.ackedThisBlock = false
	}
	if cw.ackBlock {
		n.readySendNextBlock = true
	}
	stub, decoded := n.Decoder.AddCodeword(cw.Codeword)
	n.curCodewords = append(n.curCodewords, stub)

	if !n.ackedThisBlock && len(n.curCodewords) > n.detectThreshold {
		decoded := true
		for _, c := range n.curCodewords {
			if !c.Decoded() {
				decoded = false
				break
			}
		}
		if decoded {
			n.readyReceiveNextBlock = true
			n.ackedThisBlock = true
		}
	}

	// TODO: do we consider adding it to the encoder?
	if len(decoded) > 0 {
		list := []*ldpc.Transaction{}
		for _, t := range decoded {
			list = append(list, t.Transaction)
		}
		return list
	} else {
		return nil
	}
}

func (n *node) newCodeword() codeword {
	cw := codeword{}
	if n.readyReceiveNextBlock {
		cw.ackBlock = true
		n.readyReceiveNextBlock = false
	}
	if n.readySendNextBlock && len(n.buffer) >= n.blockSize {
		cw.newBlock = true
		n.readySendNextBlock = false
		// move buffer into block
		for i := 0; i < n.blockSize; i++ {
			res := n.Encoder.AddTransaction(n.buffer[i])
			if !res {
				fmt.Println("Warning: duplicate transaction exists in window")
			}
		}
		n.buffer = n.buffer[n.blockSize:]
	}
	// TODO: what if the previous block is sent and we don't yet have the next block filled
	cw.Codeword = n.Encoder.ProduceCodeword()
	return cw
}

// updateRate should be called every 50ms
func (n *node) updateRate() {
	thisQueueLen := len(n.buffer)
	// TODO: remove hardcoded params
	deltaRate := float64(thisQueueLen - n.lastQueueLen) / float64(n.blockSize) * n.queueDiffCoeff + float64(thisQueueLen - n.targetQueueLen) / float64(n.blockSize) * n.queueTargetCoeff
	if n.sendRate + deltaRate >= n.minSendRate {
		n.sendRate = n.sendRate + deltaRate
	} else {
		n.sendRate = n.minSendRate
	}
	n.lastQueueLen = thisQueueLen
}

func newNode(config nodeConfig, decoderMemory int) *node {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(config.blockSize), 0.03, 0.5)
	n := &node{
		Encoder: ldpc.NewEncoder(experiments.TestKey, dist, config.blockSize),
		Decoder: ldpc.NewDecoder(experiments.TestKey, decoderMemory),
		curCodewords: []*ldpc.PendingCodeword{},
		buffer: []*ldpc.Transaction{},
		nodeConfig: config,
		readySendNextBlock: true,
		readyReceiveNextBlock: false,
		ackedThisBlock: false,
		lastQueueLen: 0,
		sendRate: config.minSendRate,
	}
	return n
}


func main() {
	blockSize := flag.Int("k", 500, "block size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("txgen", 600.0, "per-node transaction generation per second")
	simDuration := flag.Int("dur", 1000, "simulation duration in seconds")
	queueDiffCoeff := flag.Float64("qdiff", 0.1, "queue length diff control force")
	queueTargetCoeff := flag.Float64("qtgt", 0.05, "queue length target control force")
	targetQueueLen := flag.Int("target", 1000, "target queue length")
	minSendRate := flag.Float64("minrate", 2.0, "min codeword sending rate")
	flag.Parse()

	config := nodeConfig{
		blockSize: *blockSize,
		detectThreshold: *detectThreshold,
		queueDiffCoeff: *queueDiffCoeff,
		queueTargetCoeff: *queueTargetCoeff,
		targetQueueLen: *targetQueueLen,
		minSendRate: *minSendRate,
	}

	n1 := newNode(config, *decoderMem)
	n2 := newNode(config, *decoderMem)

	txCnt1 := 0
	txCnt2 := 0
	cwCredit1 := 0.0
	cwCredit2 := 0.0

	durMs := *simDuration * 1000
	txArrivalDist := distuv.Poisson{*transactionRate/1000.0, exprand.New(exprand.NewSource(1))}
	for tms := 0; tms <= durMs; tms += 1 {
		ts := float64(tms) / 1000.0
		if tms % 50 == 0 {
			n1.updateRate()
			n2.updateRate()
		}
		{
			prand := int(txArrivalDist.Rand())
			for i := 0; i < prand; i++ {
				tx := experiments.RandomTransaction()
				n1.addTransaction(tx)
			}
			prand = int(txArrivalDist.Rand())
			for i := 0; i < prand; i++ {
				tx := experiments.RandomTransaction()
				n2.addTransaction(tx)
			}
		}
		{
			cwCredit1 += n1.sendRate / 1000.0
			for cwCredit1 > 1.0 {
				cwCredit1 -= 1.0
				cw := n1.newCodeword()
				if cw.newBlock {
					fmt.Println(ts, "Node 1 starting new block, send rate", n1.sendRate, ", queue length", len(n1.buffer))
					txCnt2 = 0
				}
				if cw.ackBlock {
					fmt.Println(ts, "Node 1 decoded a block with", txCnt1, "txns")
				}
				list := n2.addCodeword(cw)
				txCnt2 += len(list)
			}
		}
		{
			cwCredit2 += n2.sendRate / 1000.0
			for cwCredit2 > 1.0 {
				cwCredit2 -= 1.0
				cw := n2.newCodeword()
				if cw.newBlock {
					fmt.Println(ts, "Node 2 starting new block, send rate", n2.sendRate, ", queue length", len(n2.buffer))
					txCnt1 = 0
				}
				if cw.ackBlock {
					fmt.Println(ts, "Node 2 decoded a block with", txCnt2, "txns")
				}
				list := n1.addCodeword(cw)
				txCnt1 += len(list)
			}
		}
	}
}
