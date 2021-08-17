package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"
)

func main() {
	runtimetrace := flag.String("trace", "", "write trace to `file`")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	srcSize := flag.Int("s", 0, "sender pool transation count")
	differenceSize := flag.Int("x", 0, "number of transactions that appear in the sender but not in the receiver")
	reverseDifferenceSize := flag.Int("r", 0, "number of transactions that appear in the receiver but not in the sender")
	mirrorProb := flag.Float64("m", 0, "probability that a refill transaction appears at the other end")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("p", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	refillTransaction := flag.String("f", "p(0.7)", "refill transactions at each node: c(r) for uniform arrival at rate r per codeword, p(r) for poisson arrival at rate r, empty string to disable")
	timeoutDuration := flag.Int("t", 500, "stop the experiment if no new transaction is decoded after this amount of codewords")
	timeoutCounter := flag.Int("tc", 0, "number of transactions to decode before stopping")
	degreeDistString := flag.String("d", "u(0.01)", "distribution of parity check degrees: rs(k,c,delta) for robust soliton with parameters k, c, and delta, s(k) for soliton with parameter k where k is usually the length of the encoded data, u(f) for uniform with fraction=f, b(f1, f2, p1) for bimodal with fraction=f1 with probability p1, and fraction=f2 with probability=1-p1")
	lookbackTime := flag.Uint64("l", 0, "lookback timespan of codewords, 0 for infinity")
	readConfig := flag.String("rerun", "", "read parameters from an existing output")
	flag.Parse()

	// we want to overwrite the defaults with the values from the past results
	if *readConfig != "" {
		c, err := readConfigString(*readConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*srcSize = c.SrcSize
		*differenceSize = c.DifferenceSize
		*reverseDifferenceSize = c.ReverseDifferenceSize
		*mirrorProb = c.MirrorProb
		*seed = c.Seed
		*runs = c.Runs
		*refillTransaction = c.RefillTransaction
		*timeoutDuration = c.TimeoutDuration
		*timeoutCounter = c.TimeoutCounter
		*degreeDistString = c.DegreeDistString
		*lookbackTime = c.LookbackTime
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// validate the pacer string
	_, err = NewTransactionPacer(nil, *refillTransaction)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	config := Config{
		*srcSize,
		*differenceSize,
		*reverseDifferenceSize,
		*mirrorProb,
		*seed,
		*runs,
		*refillTransaction,
		*timeoutDuration,
		*timeoutCounter,
		*degreeDistString,
		*lookbackTime,
	}
	// create output files
	var f *os.File
	var rippleF *os.File
	var pressureF *os.File
	var cwpoolF *os.File
	if *outputPrefix != "" {
		var err error
		f, err = os.Create(*outputPrefix + "-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		// dump the experiment setup
		jsonStr, err := json.Marshal(config)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(f, "# %v\n", base64.StdEncoding.EncodeToString(jsonStr))
		fmt.Fprintf(f, "# num decoded     symbols rcvd\n")

		rippleF, err = os.Create(*outputPrefix + "-ripple-size-dist.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(rippleF, "# ripple size     count\n")

		pressureF, err = os.Create(*outputPrefix + "-ntx-unique-to-p1.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(pressureF, "# iteration     unique to P1\n")
		defer pressureF.Close()

		cwpoolF, err = os.Create(*outputPrefix + "-p2-codeword-pool.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(cwpoolF, "# iteration     P2 unreleased cw\n")
		defer cwpoolF.Close()
	}

	if *runtimetrace != "" {
		f, err := os.Create(*runtimetrace)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()

		if err := trace.Start(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer trace.Stop()
	}

	// start the profile
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	var chs []chan int    // channels for the interation-#decoded result
	var rippleCh chan int // channel to collect the ripple sizes
	if rippleF != nil {
		rippleCh = make(chan int, 1000)
	}
	var pressureChs []chan int // channel to collect num of undecoded transactions
	var cwpoolChs []chan int   // channel to collect the number of unreleased codewords
	procwg := &sync.WaitGroup{}
	for i := 0; i < *runs; i++ {
		procwg.Add(1)
		var ch chan int
		if f != nil {
			ch = make(chan int, 1000)
		}
		chs = append(chs, ch)
		var pressureCh chan int
		if pressureF != nil {
			pressureCh = make(chan int, 1000)
		}
		pressureChs = append(pressureChs, pressureCh)
		var cwpoolCh chan int
		if cwpoolF != nil {
			cwpoolCh = make(chan int, 1000)
		}
		cwpoolChs = append(cwpoolChs, cwpoolCh)
		sd := rand.Int63()
		go func(s int64) {
			if ch != nil {
				defer close(ch)
			}
			if pressureCh != nil {
				defer close(pressureCh)
			}
			if cwpoolCh != nil {
				defer close(cwpoolCh)
			}
			defer procwg.Done()
			err := runExperiment(*srcSize, *differenceSize, *reverseDifferenceSize, *timeoutDuration, *timeoutCounter, *refillTransaction, *mirrorProb, ch, rippleCh, pressureCh, cwpoolCh, *degreeDistString, *lookbackTime, s)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}(sd)
	}

	// monitor and dump to files
	wg := &sync.WaitGroup{}
	// for each tx idx, range over res channels to collect data and dump to file
	if f != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(chs, dch)
			for d := range dch {
				if f != nil {
					fmt.Fprintf(f, "%v        %v\n", idx, d)
				}
				fmt.Printf("Iteration=%v, transactions=%v\n", d, idx)
				idx += 1
			}
		}()
	}
	// collect and dump ripple size distribution
	if rippleF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collectDumpHistogram(rippleF, rippleCh)
		}()
	}
	// collect and dump num of transactions unique to p1
	if pressureF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(pressureChs, dch)
			for d := range dch {
				fmt.Fprintf(pressureF, "%v        %v\n", idx, d)
				idx += 1
			}
		}()
	}
	if cwpoolF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dch := make(chan int, 1000)
			idx := 0
			go collectAverage(cwpoolChs, dch)
			for d := range dch {
				fmt.Fprintf(cwpoolF, "%v        %v\n", idx, d)
				idx += 1
			}
		}()
	}

	procwg.Wait()
	if rippleCh != nil {
		close(rippleCh)
	}
	wg.Wait()

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return
}

type StuckError struct {
	nid int
}

func (e StuckError) Error() string {
	return fmt.Sprintf("node %v is stuck", e.nid)
}

type TransactionCountError struct {
	nid int
}

func (e TransactionCountError) Error() string {
	return fmt.Sprintf("transaction count limit reached at node %v", e.nid)
}

func runExperiment(s, d, r, tout, tcnt int, refill string, mirror float64, res, ripple, diff, cwpool chan int, dist string, lookback uint64, seed int64) error {
	if lookback == 0 {
		lookback = math.MaxUint64
	}
	rng := rand.New(rand.NewSource(seed))
	var nodes []*node
	for nidx := 0; nidx < 2; nidx++ {
		dt, err := NewDistribution(rng, dist, d+r)
		if err != nil {
			return err
		}
		pc, err := NewTransactionPacer(rng, refill)
		if err != nil {
			return err
		}
		p := newNode(dt, rng, pc, lookback)
		nodes = append(nodes, p)
	}
	nodes[0].connectTo(nodes[1])
	p1tx := nodes[0].fillInitTransaction(nil, 0, s)
	nodes[1].fillInitTransaction(p1tx, s-d, r)

	if res != nil {
		res <- 0 // at iteration 0, we have decoded 0 transactions
	}
	// start sending codewords from p1 to p2
	// prepare the counters
	totalTx := 0
	i := 0 // iteration counter
	for {
		i += 1
		for nidx := range nodes {
			nodes[nidx].sendCodewords()
		}
		for nidx := range nodes {
			updated := nodes[nidx].tryDecode()
			if nidx == 1 {
				if res != nil {
					for cnt := 0; cnt < updated; cnt++ {
						res <- i
					}
				}
				if ripple != nil {
					ripple <- updated
				}
				if diff != nil {
					diff <- totalTx - nodes[nidx].txPoolSize()
				}
				if cwpool != nil {
					sum := 0
					for _, peer := range nodes[nidx].PeerStates {
						sum += peer.NumPendingCodewords()
					}
					cwpool <- sum
				}
			}
			// stop if node is stuck
			if int(nodes[nidx].Seq-nodes[nidx].lastAct) > tout {
				return StuckError{nidx}
			}
			if tcnt != 0 && tcnt <= nodes[nidx].decoded {
				return TransactionCountError{nidx}
			}
			// add transactions to pools
			nadd := nodes[nidx].transactionPacer.tick()
			for cnt := 0; cnt < nadd; cnt++ {
				t := nodes[nidx].getRandomTransaction()
				nodes[nidx].AddLocalTransaction(t)
				totalTx += 1
				if rng.Float64() < mirror {
					for n2idx := range nodes {
						if n2idx != nidx {
							nodes[n2idx].AddLocalTransaction(t)
						}
					}
				}
			}
		}
	}
}

type Config struct {
	SrcSize               int
	DifferenceSize        int
	ReverseDifferenceSize int
	MirrorProb            float64
	Seed                  int64
	Runs                  int
	RefillTransaction     string
	TimeoutDuration       int
	TimeoutCounter        int
	DegreeDistString      string
	LookbackTime          uint64
}

func readConfigString(prefix string) (Config, error) {
	config := Config{}
	// read the first line and strip "# " to get the base64 encoded json
	ef, err := os.Open(prefix + "-mean-iter-to-decode.dat")
	if err != nil {
		return config, err
	}
	defer ef.Close()
	scanner := bufio.NewScanner(ef)
	scanner.Scan()
	b64 := scanner.Text()
	data, err := base64.StdEncoding.DecodeString(b64[2:]) // strip the prefix "# "
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

func collectDumpHistogram(f *os.File, ch chan int) {
	hist := make(map[int]int)
	maxd := 0
	tot := 0
	totPrd := 0
	for d := range ch {
		hist[d] += 1
		if maxd < d {
			maxd = d
		}
		tot += 1
		totPrd += d
	}
	cdf := 0
	for i := 0; i <= maxd; i++ {
		if val, there := hist[i]; there {
			cdf += val * i
			fmt.Fprintf(f, "%v         %v           %v\n", i, float64(val)/float64(tot), float64(cdf)/float64(totPrd))
		}
	}
}

func collectAverage(chs []chan int, out chan int) {
	defer close(out)
	for {
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
			out <- d / len(chs)
		}
	}
}
