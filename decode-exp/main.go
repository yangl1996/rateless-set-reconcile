package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"flag"
	"os"
	"fmt"
	"time"
	"math/rand"
)

func main() {
	thresholdInt := flag.Int("t", 20, "threshold to filter txs in a codeword, must be in [0, 256)")
	srcSize := flag.Int("s", 10000, "source pool transation count")
	destSize := flag.Int("d", 10000, "destination pool transaction count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the source but not in the destination")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	flag.Parse()
	var threshold byte
	if *thresholdInt > 255 || *thresholdInt < 0 {
		fmt.Println("threshold must be in [0, 256)")
		os.Exit(1)
	} else {
		threshold = byte(*thresholdInt)
	}
	if *destSize < *srcSize - *differenceSize {
		fmt.Println("destination pool must be no smaller than source pool minues the difference (d >= s-x)")
		os.Exit(1)
	}
	if *seed == 0 {
		rand.Seed(time.Now().UTC().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	p1, err := buildRandomPool(*srcSize)
	if err != nil {
		fmt.Println("failed to build source pool")
		os.Exit(1)
	}
	p2, err := copyPoolWithDifference(p1, *destSize, *differenceSize)
	if err != nil {
		fmt.Println("failed to build dest pool")
		os.Exit(2)
	}

	// start sending codewords from p1 to p2
	i := 0
	for ;; {
		i += 1
		salt := [4]byte{}	// use 32-bit salt, should be enough
		rand.Read(salt[:])
		c := p1.ProduceCodeword(salt[:], threshold)
		p2.InputCodeword(c)
		p2.TryDecode()
		fmt.Printf("Iteration=%v, codewords=%v, transactions=%v\n", i, len(p2.Codewords), len(p2.Transactions))
		if len(p2.Transactions) == *destSize + *differenceSize {
			break
		}
	}
	// compare if p1 is a subset of p2; we take a shortcut by checking if the last differenceSize elements in p2
	// exist in p1
	for i := *destSize; i < len(p2.Transactions); i++ {
		if !p1.Exists(p2.Transactions[i].Transaction) {
			fmt.Println("found decoded transaction in p2 that does not appear in p1")
			os.Exit(1)
		}
	}
	return
}

func buildRandomPool(n int) (*ldpc.TransactionPool, error) {
	p, err := ldpc.NewTransactionPool()
        if err != nil {
                return nil, err
        }
        for i := 0; i < n; i++ {
                d := [ldpc.TxSize]byte{}
                rand.Read(d[:])
                p.AddTransaction(d)
        }
        return p, nil
}

// copyPoolWithDifference copies the transactions from src excluding the last x into a new pool, and
// fills the new pool with new, random transactions to a total of n.
func copyPoolWithDifference(src *ldpc.TransactionPool, n int, x int) (*ldpc.TransactionPool, error) {
	p, err := ldpc.NewTransactionPool()
	if err != nil {
		return nil, err
	}
	i := 0
	for ; i < len(src.Transactions)-x; i++ {
		p.AddTransaction(src.Transactions[i].Transaction)
	}
	for ; i < n; i++ {
                d := [ldpc.TxSize]byte{}
                rand.Read(d[:])
                p.AddTransaction(d)
	}
	return p, nil
}
