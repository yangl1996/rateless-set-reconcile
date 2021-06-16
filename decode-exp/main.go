package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"flag"
	"os"
	"fmt"
	"time"
	"math/rand"
	"math"
	"math/big"
	"errors"
)

func main() {
	thresholdFloat := flag.Float64("t", 0.05, "threshold to filter txs in a codeword, must be within [0, 1]")
	srcSize := flag.Int("s", 10000, "source pool transation count")
	destSize := flag.Int("d", 9900, "destination pool transaction count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the source but not in the destination")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("r", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	noTermOut := flag.Bool("q", false, "do not print log to terminal")
	refillTransaction := flag.Int("f", 100, "refill a transaction immediately after the destination pool has decoded one")
	flag.Parse()
	var threshold uint64
	if *thresholdFloat > 1 || *thresholdFloat < 0 {
		fmt.Println("threshold must be in [0, 1]")
		os.Exit(1)
	} else {
		trat := new(big.Float).SetFloat64(*thresholdFloat)
		maxt := new(big.Float).SetUint64(math.MaxUint64)
		threshold, _ = new(big.Float).Mul(trat, maxt).Uint64()
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

	var chs []chan int
	for i := 0; i < *runs; i++ {
		ch := make(chan int, *differenceSize)
		chs = append(chs, ch)
		go func() {
			err := runExperiment(*srcSize, *destSize, *differenceSize, *refillTransaction, threshold, ch)
			if err != nil {
				fmt.Println(err)
			}
		}()
	}

	var f *os.File
	if *outputPrefix != "" {
		var err error
		f, err = os.Create(*outputPrefix+"-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintf(f, "# |src|=%v, |dst|=%v, refill=%v, diff=%v, frac=%v\n", *srcSize, *destSize, *refillTransaction, *differenceSize, *thresholdFloat)
		fmt.Fprintf(f, "# num decoded     symbols rcvd\n")
	}
	// for each tx idx, range over res channels to collect data and dump to file
	for idx := 0;; idx++ {
		nClosed := 0
		d := 0
		for _, ch := range chs {
			td, more := <-ch
			if more {
				d += td
			} else {
				nClosed += 1
			}
		}
		if nClosed >= len(chs) {
			return
		} else if nClosed == 0 {
			if f != nil {
				fmt.Fprintf(f, "%v        %v\n", idx, d / len(chs))
			}
			if !*noTermOut {
				fmt.Printf("Iteration=%v, transactions=%v\n", d/len(chs), idx)
			}
		} else {
			fmt.Println(nClosed, "of", *runs, "runs have stopped, waiting for all to stop")
		}
	}

	return
}

// runExperiment runs the experiment and returns an array of data. The i-th element in the array is the iteration
// where the i-th item is decoded.
func runExperiment(s, d, x, f int, th uint64, res chan int) error {
	defer close(res)	// close when the experiment ends
	p1, err := buildRandomPool(s)
	if err != nil {
		return err
	}
	p2, err := copyPoolWithDifference(p1, d, x)
	if err != nil {
		return err
	}

	res <- 0 // at iteration 0, we have decoded 0 transactions
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
		if len(p2.Transactions) > last {
			for cnt := last; cnt < len(p2.Transactions); cnt++ {
				// check if the tx that p2 just decoded is actually in p1
				if !p1.Exists(p2.Transactions[cnt].Transaction) {
					return errors.New(fmt.Sprint("dest decoded a bogus transaction"))
				} else {
					res <- i
				}
				if f > 0 {
					p1.AddTransaction(getRandomTransaction())
					f -= 1
				}
			}
			last = len(p2.Transactions)
		}
		if len(p2.Transactions) == d + x {
			break
		}
	}
	return nil
}

func getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	rand.Read(d[:])
	return ldpc.NewTransaction(d)
}

func buildRandomPool(n int) (*ldpc.TransactionPool, error) {
	p, err := ldpc.NewTransactionPool()
        if err != nil {
                return nil, err
        }
        for i := 0; i < n; i++ {
                p.AddTransaction(getRandomTransaction())
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
                p.AddTransaction(getRandomTransaction())
	}
	return p, nil
}
