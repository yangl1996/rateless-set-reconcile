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
)

const (
	uniform = iota
	solition
)

func fracToThreshold(f float64) uint64 {
	trat := new(big.Float).SetFloat64(f)
	maxt := new(big.Float).SetUint64(math.MaxUint64)
	threshold, _ := new(big.Float).Mul(trat, maxt).Uint64()
	return threshold
}

func main() {
	thresholdFloat := flag.Float64("t", 0.01, "threshold to filter txs in a codeword if degree distribution is uniform, must be within [0, 1]")
	solitionK := flag.Uint64("k", 5, "parameter 'k' if degree distribution is solution, must be greater than 0")
	srcSize := flag.Int("s", 10000, "sender pool transation count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the sender but not in the receiver")
	reverseDifferenceSize := flag.Int("r", 0, "number of transactions that appear in the receiver but not in the sender")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("p", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	noTermOut := flag.Bool("q", false, "do not print log to terminal (quiet)")
	refillTransaction := flag.Int("f", 100, "refill a transaction immediately after the destination pool has decoded one")
	degreeDistString := flag.String("dist", "solition", "distribution of parity check degrees (solition, uniform)")
	flag.Parse()
	var degreeDist int
	switch *degreeDistString {
	case "uniform":
		degreeDist = uniform
		if *thresholdFloat > 1 || *thresholdFloat < 0 {
			fmt.Println("threshold must be in [0, 1]")
			os.Exit(1)
		}
	case "solition":
		degreeDist = solition
		if *solitionK <= 0 {
			fmt.Println("solition parameter K must be greater than 0")
			os.Exit(1)
		}
	default:
		fmt.Println("undefined distribution")
		os.Exit(1)
	}
	threshold := fracToThreshold(*thresholdFloat)
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
			err := runExperiment(*srcSize, *differenceSize, *reverseDifferenceSize, *refillTransaction, threshold, *solitionK, ch, degreeDist)
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
		fmt.Fprintf(f, "# |src|=%v, |S\\D|=%v, |D\\S|=%v, refill=%v, dist=%s, frac=%v\n", *srcSize, *differenceSize, *reverseDifferenceSize, *refillTransaction, *degreeDistString, *thresholdFloat)
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
func runExperiment(s, d, r, f int, th, k uint64, res chan int, dist int) error {
	defer close(res)	// close when the experiment ends
	p1, err := buildRandomPool(s)
	if err != nil {
		return err
	}
	p2, err := copyPoolWithDifference(p1, s-d+r, d)
	if err != nil {
		return err
	}

	so := ldpc.NewSolition(k)	// solitition distribution, unused if dist!=solition


	res <- 0 // at iteration 0, we have decoded 0 transactions
	// start sending codewords from p1 to p2
	i := 0
	last := len(p2.Transactions)
	lastUs := len(p2.UniqueToUs)
	for ;; {
		i += 1
		salt := [4]byte{}	// use 32-bit salt, should be enough
		rand.Read(salt[:])
		var c ldpc.Codeword
		switch dist {
		case uniform:
			c = p1.ProduceCodeword(salt[:], th)	// if uniform, just use the given threshold integer
		case solition:
			sk := so.Uint64()
			// given a draw, sk, from solition, we want to set the threshold such that sk differences are xor'ed into the codeword
			// we use d+r to "estimate" the set difference, so we want the fraction to be sk/(d+r)
			// in the real world, we can only get an estimation of the set diff, because d, r are unknown
			th := fracToThreshold(float64(sk)/float64(d+r))
			c = p1.ProduceCodeword(salt[:], th)
		}
		p2.InputCodeword(c)
		p2.TryDecode()
		for cnt := 0; cnt < len(p2.Transactions)-last; cnt++ {
			res <- i
			if f > 0 {
				p1.AddTransaction(getRandomTransaction())
				f -= 1
			}
		}
		for cnt := 0; cnt < len(p2.UniqueToUs)-lastUs; cnt++ {
			res <- i
		}
		last = len(p2.Transactions)
		lastUs = len(p2.UniqueToUs)
		if len(p2.Transactions) == s+r {
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
	for tx, _ := range src.Transactions {
		p.AddTransaction(tx.Transaction)
		i += 1
		if i >= len(src.Transactions)-x {
			break
		}
	}
	for ; i < n; i++ {
                p.AddTransaction(getRandomTransaction())
	}
	return p, nil
}
