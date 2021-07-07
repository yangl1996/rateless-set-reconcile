package main

import (
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
	differenceSize := flag.Int("x", 1000, "number of transactions that appear in the sender but not in the receiver")
	reverseDifferenceSize := flag.Int("r", 0, "number of transactions that appear in the receiver but not in the sender")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("p", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	noTermOut := flag.Bool("q", false, "do not print log to terminal (quiet)")
	refillTransaction := flag.String("f", "", "refill transactions at the sender: c(r) for uniform arrival at rate r per codeword, empty string to disable")
	timeoutDuration := flag.Int("t", 500, "stop the experiment if no new transaction is decoded after this amount of codewords")
	degreeDistString := flag.String("d", "s(1000)", "distribution of parity check degrees: rs(k,c,delta) for robust soliton with parameters k, c, and delta, s(k) for soliton with parameter k where k is usually the length of the encoded data, u(f) for uniform with fraction=f")
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
		*timeoutDuration = c.TimeoutDuration
		*degreeDistString = c.DegreeDistString
		// we then parse the command line args again, so that only the ones explicitly given
		// in the command line will be overwritten
		flag.Parse()
	}

	if *seed == 0 {
		*seed = time.Now().UTC().UnixNano()
	}
	rand.Seed(*seed)
	// validate the syntax of the dist string
	_, err := NewDistribution(nil, *degreeDistString, *differenceSize+*reverseDifferenceSize)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// validate the pacer string
	_, err = NewTransactionPacer(*refillTransaction)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	config := Config {
		*srcSize,
		*differenceSize,
		*reverseDifferenceSize,
		*seed,
		*runs,
		*refillTransaction,
		*timeoutDuration,
		*degreeDistString,
	}
	var chs []chan int	// channels for the interation-#decoded result
	degreeCh := make(chan int, 1000)	// channel to collect the degree of codewords
	for i := 0; i < *runs; i++ {
		ch := make(chan int, *differenceSize)
		chs = append(chs, ch)
		sd := rand.Int63()
		go func(s int64) {
			err := runExperiment(*srcSize, *differenceSize, *reverseDifferenceSize, *timeoutDuration, *refillTransaction, ch, degreeCh, *degreeDistString, s)
			if err != nil {
				fmt.Println(err)
			}
		}(sd)
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

func runExperiment(s, d, r, tout int, refill string, res, degree chan int, dist string, seed int64) error {
	defer close(res)	// close when the experiment ends
	rng := rand.New(rand.NewSource(seed))
	dist1, err := NewDistribution(rng, dist, d+r)
	if err != nil {
		return err
	}
	pacer1, err := NewTransactionPacer(refill)
	if err != nil {
		return err
	}
	p1, err := newNode(nil, 0, s, dist1, rng, pacer1)
	if err != nil {
		return err
	}
	rng2 := rand.New(rand.NewSource(seed+1000))
	dist2, err := NewDistribution(rng2, dist, d+r)
	if err != nil {
		return err
	}
	pacer2, err := NewTransactionPacer(refill)
	if err != nil {
		return err
	}
	p2, err := newNode(p1.TransactionPool, s-d, r, dist2, rng2, pacer2)
	if err != nil {
		return err
	}

	res <- 0 // at iteration 0, we have decoded 0 transactions
	// start sending codewords from p1 to p2
	i := 0	// iteration counter
	received := len(p2.Transactions)
	lastAct := 0
	for ;; {
		i += 1
		c := p1.produceCodeword()
		c2 := p2.produceCodeword()
		degree <- c.Counter
		p2.InputCodeword(c)
		p2.TryDecode()
		p1.InputCodeword(c2)
		p1.TryDecode()
		thisBatch := len(p2.Transactions) - received
		for cnt := 0; cnt < thisBatch; cnt++ {
			res <- i
			lastAct = i
		}
		if i - lastAct > tout {
			return nil
		}
		nadd := p1.pacer.tick()
		for cnt := 0; cnt < nadd; cnt++ {
				p1.AddTransaction(p1.getRandomTransaction())
		}
		nadd = p2.pacer.tick()
		for cnt := 0; cnt < nadd; cnt++ {
				p2.AddTransaction(p2.getRandomTransaction())
		}
		received = len(p2.Transactions)
	}
}

type Config struct {
	SrcSize int
	DifferenceSize int
	ReverseDifferenceSize int
	Seed int64
	Runs int
	RefillTransaction string
	TimeoutDuration int
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
