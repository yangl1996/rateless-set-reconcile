package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/rateless-set-reconcile/experiments"
	"github.com/yangl1996/soliton"
	"math/rand"
	"flag"
)

type codeword struct {
	*ldpc.Codeword
	newBlock bool
	ackBlock bool
}

type node struct {
	*ldpc.Encoder
	*ldpc.Decoder
	curCodewords []*ldpc.PendingCodeword
	buffer []*ldpc.Transaction
	
	// parameters
	blockSize int
	detectThreshold int

	// flags that affect the next codeword
	readySendNextBlock bool
	readyReceiveNextBlock bool
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
	}
	if cw.ackBlock {
		n.readySendNextBlock = true
	}
	stub, decoded := n.Decoder.AddCodeword(cw.Codeword)
	n.curCodewords = append(n.curCodewords, stub)

	if len(n.curCodewords) > n.detectThreshold {
		decoded := true
		for _, c := range n.curCodewords {
			if !c.Decoded() {
				decoded = false
				break
			}
		}
		if decoded {
			n.readyReceiveNextBlock = true
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

func newNode(blockSize int, decoderMemory int, detectThreshold int) *node {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(blockSize), 0.03, 0.5)
	n := &node{
		Encoder: ldpc.NewEncoder(experiments.TestKey, dist, blockSize),
		Decoder: ldpc.NewDecoder(experiments.TestKey, decoderMemory),
		curCodewords: []*ldpc.PendingCodeword{},
		buffer: []*ldpc.Transaction{},
		blockSize: blockSize,
		detectThreshold: detectThreshold,
		readySendNextBlock: true,
		readyReceiveNextBlock: false,
	}
	return n
}


func main() {
	blockSize := flag.Int("k", 500, "block size")
	decoderMem := flag.Int("mem", 1000000, "decoder memory")
	detectThreshold := flag.Int("th", 50, "detector threshold")
	transactionRate := flag.Float64("rate", 600.0, "per-node transaction generation per second")
	simDuration := flag.Int("dur", 1000, "simulation duration in seconds")
	flag.Parse()

	n1 := newNode(*blockSize, *decoderMem, *detectThreshold)
	n2 := newNode(*blockSize, *decoderMem, *detectThreshold)

	durMs := *simDuration * 1000
	newTxProbPerMs := *transactionRate / 1000.0
	for tms := 0; tms <= durMs; tms += 1 {
		if rand.Float64() < newTxProbPerMs {
			tx := experiments.RandomTransaction()
			n1.addTransaction(tx)
		}
		if rand.Float64() < newTxProbPerMs {
			tx := experiments.RandomTransaction()
			n2.addTransaction(tx)
		}
		cw := n1.newCodeword()
		if cw.newBlock {
			fmt.Println("Node 1 starting new block")
		}
		if cw.ackBlock {
			fmt.Println("Node 1 decoded a block")
		}
		list := n2.addCodeword(cw)
		if len(list) > 0 {
			fmt.Println("Node 2 decoded", len(list), "transactions")
		}
		cw = n2.newCodeword()
		if cw.newBlock {
			fmt.Println("Node 2 starting new block")
		}
		if cw.ackBlock {
			fmt.Println("Node 2 decoded a block")
		}

		list = n1.addCodeword(cw)
		if len(list) > 0 {
			fmt.Println("Node 1 decoded", len(list), "transactions")
		}
	}
}
