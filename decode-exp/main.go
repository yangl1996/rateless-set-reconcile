package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"flag"
	"os"
	"fmt"
	"math/rand"
)

func main() {
	thresholdInt := flag.Int("t", 20, "threshold to filter txs in a codeword, must be in [0, 256)")
	srcSize := flag.Int("s", 10000, "source pool transation count")
	destSize := flag.Int("d", 10000, "destination pool transaction count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the source but not in the destination")
	flag.Parse()
	if *thresholdInt > 255 || *thresholdInt < 0 {
		fmt.Println("threshold must be in [0, 256)")
		os.Exit(1)
	}
	if *destSize < *srcSize - *differenceSize {
		fmt.Println("destination pool must be no smaller than source pool minues the difference (d >= s-x)")
		os.Exit(1)
	}

	p1, err := buildRandomPool(*srcSize)
	if err != nil {
		fmt.Println("failed to build source pool")
		os.Exit(1)
	}
	_, err = copyPoolWithDifference(p1, *destSize, *differenceSize)
	if err != nil {
		fmt.Println("failed to build dest pool")
		os.Exit(2)
	}
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
