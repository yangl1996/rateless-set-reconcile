package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"flag"
	"os"
	"fmt"
	"time"
	"math/rand"
	"errors"
)

func main() {
	thresholdInt := flag.Int("t", 20, "threshold to filter txs in a codeword, must be in [0, 256)")
	srcSize := flag.Int("s", 10000, "source pool transation count")
	destSize := flag.Int("d", 9900, "destination pool transaction count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the source but not in the destination")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("r", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "", "output data path prefix, no output if empty")
	flag.Parse()
	var threshold byte
	if *thresholdInt > 255 || *thresholdInt < 0 {
		fmt.Println("threshold must be in [0, 256)")
		os.Exit(1)
	} else {
		threshold = byte(*thresholdInt)
	}
	if *destSize < *srcSize - *differenceSize {
		fmt.Println("destination pool must be no smaller than source pool minus the difference (d >= s-x)")
		os.Exit(1)
	}
	if *seed == 0 {
		rand.Seed(time.Now().UTC().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	ch := make(chan []int)
	for i := 0; i < *runs; i++ {
		go func() {
			res, err := runExperiment(*srcSize, *destSize, *differenceSize, threshold, *runs==1)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			} else {
				ch <- res
			}
		}()
	}
	var d []int
	for i := 0; i < *runs; i++ {
		nd := <-ch	// new data
		if d == nil {
			d = nd
		} else {
			for idx, _ := range d {
				if idx < len(nd) {
					d[idx] += nd[idx]
				}
			}
		}
	}

	// output data
	if *outputPrefix != "" {
		f, err := os.Create(*outputPrefix+"-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintf(f, "# num decoded     symbols rcvd\n")
		for idx, rnd := range d {
			fmt.Fprintf(f, "%v        %v\n", idx, rnd / *runs)
		}
	}
	return
}

// runExperiment runs the experiment and returns an array of data. The i-th element in the array is the iteration
// where the i-th item is decoded.
func runExperiment(s, d, x int, th byte, log bool) ([]int, error) {
	p1, err := buildRandomPool(s)
	if err != nil {
		return nil, err
	}
	p2, err := copyPoolWithDifference(p1, d, x)
	if err != nil {
		return nil, err
	}

	res := []int{}
	res = append(res, 0)	// at iteration 0, we have decoded 0 transactions
	// start sending codewords from p1 to p2
	i := 0
	last := len(p2.Transactions)
	for ;; {
		i += 1
		salt := [4]byte{}	// use 32-bit salt, should be enough
		rand.Read(salt[:])
		c := p1.ProduceCodeword(salt[:], th)
		p2.InputCodeword(c)
		p2.TryDecode()
		if log {
			fmt.Printf("Iteration=%v, codewords=%v, transactions=%v\n", i, len(p2.Codewords), len(p2.Transactions))
		}
		if len(p2.Transactions) > last {
			for cnt := last; cnt < len(p2.Transactions); cnt++ {
				res = append(res, i)
			}
			last = len(p2.Transactions)
		}
		if len(p2.Transactions) == d + x {
			break
		}
	}
	// compare if p1 is a subset of p2; we take a shortcut by checking if the last differenceSize elements in p2
	// exist in p1
	nc := 0
	for i := d; i < len(p2.Transactions); i++ {
		if !p1.Exists(p2.Transactions[i].Transaction) {
			nc += 1
		}
	}
	if nc > 0 {
		return nil, errors.New(fmt.Sprint("found", nc, "decoded transactions in p2 that does not appear in p1"))
	}
	return res, nil
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
