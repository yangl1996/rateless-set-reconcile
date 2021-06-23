package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"flag"
	"os"
	"fmt"
	"time"
	"math/rand"
	"sync"
	"encoding/json"
	"encoding/base64"
	"bufio"
)

func main() {
	srcSize := flag.Int("s", 10000, "sender pool transation count")
	differenceSize := flag.Int("x", 100, "number of transactions that appear in the sender but not in the receiver")
	reverseDifferenceSize := flag.Int("r", 0, "number of transactions that appear in the receiver but not in the sender")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("p", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	noTermOut := flag.Bool("q", false, "do not print log to terminal (quiet)")
	refillTransaction := flag.Int("f", 100, "refill a transaction immediately after the destination pool has decoded one")
	degreeDistString := flag.String("d", "u(0.01)", "distribution of parity check degrees (rs(k,c,delta) for robust soliton with parameters k, c, and delta, s(k) for soliton with parameter k where k is usually the length of the encoded data, u(f) for uniform with fraction=f)")
	readConfig := flag.String("rerun", "", "read parameters from an existing output")
	flag.Parse()

	// we want to overwrite the defaults with the values from the past results
	if *readConfig != "" {
		c, err := readConfigString(*readConfig)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		*srcSize = c.SrcSize
		*differenceSize = c.DifferenceSize
		*reverseDifferenceSize = c.ReverseDifferenceSize
		*seed = c.Seed
		*runs = c.Runs
		*refillTransaction = c.RefillTransaction
		*degreeDistString = c.DegreeDistString
		// we then parse the command line args again, so that only the ones explicitly given
		// in the command line will be overwritten
		flag.Parse()
	}

	degreeDist, err := NewDistribution(*degreeDistString, *differenceSize+*reverseDifferenceSize)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if *seed == 0 {
		rand.Seed(time.Now().UTC().UnixNano())
	} else {
		rand.Seed(*seed)
	}
	// TODO: deal with it
	if *refillTransaction != 0 {
		fmt.Println("refilling is disabled because of a bug: if the receiver decodes a transaction before it decodes all original transactions, it might think the refilled transaction to be unique to itself and do strange things")
		os.Exit(1)
	}

	config := Config {
		*srcSize,
		*differenceSize,
		*reverseDifferenceSize,
		*seed,
		*runs,
		*refillTransaction,
		*degreeDistString,
	}
	var chs []chan int	// channels for the interation-#decoded result
	degreeCh := make(chan int, 1000)	// channel to collect the degree of codewords
	for i := 0; i < *runs; i++ {
		ch := make(chan int, *differenceSize)
		chs = append(chs, ch)
		go func() {
			err := runExperiment(*srcSize, *differenceSize, *reverseDifferenceSize, *refillTransaction, ch, degreeCh, degreeDist)
			if err != nil {
				fmt.Println(err)
			}
		}()
	}

	var f *os.File
	var degreeF *os.File
	if *outputPrefix != "" {
		var err error
		f, err = os.Create(*outputPrefix+"-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer f.Close()
		// dump the experiment setup
		jsonStr, err := json.Marshal(config)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Fprintf(f, "# %v\n", base64.StdEncoding.EncodeToString(jsonStr))
		fmt.Fprintf(f, "# |src|=%v, |S\\D|=%v, |D\\S|=%v, refill=%v, dist=%s\n", *srcSize, *differenceSize, *reverseDifferenceSize, *refillTransaction, *degreeDistString)
		fmt.Fprintf(f, "# num decoded     symbols rcvd\n")

		degreeF, err = os.Create(*outputPrefix+"-codeword-degree-dist.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Fprintf(degreeF, "# codeword degree     count\n")
		defer degreeF.Close()
	}

	// monitor and dump to files
	wg := &sync.WaitGroup{}
	// for each tx idx, range over res channels to collect data and dump to file
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(degreeCh)	// we are sure that after all res channels close, no gorountine will write to degreeCh
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
	}()
	// collect and dump codeword degree distribution
	wg.Add(1)
	go func() {
		hist := make(map[int]int)
		maxd := 0
		tot := 0
		defer wg.Done()
		for d := range degreeCh {
			hist[d] += 1
			if maxd < d {
				maxd = d
			}
			tot += 1
		}
		if degreeF != nil {
			for i:=0; i <=maxd; i++ {
				if val, there := hist[i]; there {
					fmt.Fprintf(degreeF, "%v         %v\n", i, float64(val)/float64(tot))
				}
			}
		}
	}()

	wg.Wait()
	return
}

// runExperiment runs the experiment and returns an array of data. The i-th element in the array is the iteration
// where the i-th item is decoded.
func runExperiment(s, d, r, f int, res, degree chan int, dist thresholdPicker) error {
	defer close(res)	// close when the experiment ends
	p1, err := buildRandomPool(s)
	if err != nil {
		return err
	}
	p2, err := copyPoolWithDifference(p1, s-d+r, d)
	if err != nil {
		return err
	}

	res <- 0 // at iteration 0, we have decoded 0 transactions
	// start sending codewords from p1 to p2
	i := 0
	last := len(p2.Transactions)
	lastUs := len(p2.UniqueToUs)
	for ;; {
		i += 1
		salt := [4]byte{}	// use 32-bit salt, should be enough
		rand.Read(salt[:])
		c := p1.ProduceCodeword(salt[:], dist.generate())
		degree <- c.Counter
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

type Config struct {
	SrcSize int
	DifferenceSize int
	ReverseDifferenceSize int
	Seed int64
	Runs int
	RefillTransaction int
	DegreeDistString string
}


func readConfigString(prefix string) (Config, error) {
	config := Config{}
	// read the first line and strip "# " to get the base64 encoded json
	ef, err := os.Open(prefix+"-mean-iter-to-decode.dat")
	if err != nil {
		return config, err
	}
	defer ef.Close()
	scanner := bufio.NewScanner(ef)
	scanner.Scan()
	b64 := scanner.Text()
	data, err := base64.StdEncoding.DecodeString(b64[2:len(b64)])	// strip the prefix "# "
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}
