package main

import (
	"runtime"
	"runtime/pprof"
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
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	srcSize := flag.Int("s", 0, "sender pool transation count")
	differenceSize := flag.Int("x", 0, "number of transactions that appear in the sender but not in the receiver")
	reverseDifferenceSize := flag.Int("r", 0, "number of transactions that appear in the receiver but not in the sender")
	seed := flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
	runs := flag.Int("p", 1, "number of parallel runs")
	outputPrefix := flag.String("out", "out", "output data path prefix, no output if empty")
	noTermOut := flag.Bool("q", false, "do not print log to terminal (quiet)")
	refillTransaction := flag.String("f", "p(0.7)", "refill transactions at each node: c(r) for uniform arrival at rate r per codeword, p(r) for poisson arrival at rate r, n(c) for keeping a constant number c of transactions, empty string to disable")
	timeoutDuration := flag.Int("t", 500, "stop the experiment if no new transaction is decoded after this amount of codewords")
	timeoutCounter := flag.Int("tc", 0, "number of transactions to decode before stopping")
	degreeDistString := flag.String("d", "u(0.01)", "distribution of parity check degrees: rs(k,c,delta) for robust soliton with parameters k, c, and delta, s(k) for soliton with parameter k where k is usually the length of the encoded data, u(f) for uniform with fraction=f, b(f1, f2, p1) for bimodal with fraction=f1 with probability p1, and fraction=f2 with probability=1-p1")
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
		*timeoutCounter = c.TimeoutCounter
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
	_, err = NewTransactionPacer(nil, *refillTransaction)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// start the profile
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Println("could not create CPU profile: ", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Println("could not start CPU profile: ", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	config := Config {
		*srcSize,
		*differenceSize,
		*reverseDifferenceSize,
		*seed,
		*runs,
		*refillTransaction,
		*timeoutDuration,
		*timeoutCounter,
		*degreeDistString,
	}
	var chs []chan int	// channels for the interation-#decoded result
	degreeCh := make(chan int, 1000)	// channel to collect the degree of codewords
	var pressureChs []chan int		// channel to collect num of transactions
	var cwpoolChs []chan int
	for i := 0; i < *runs; i++ {
		ch := make(chan int, 1000)
		chs = append(chs, ch)
		pressureCh := make(chan int, 1000)
		pressureChs = append(pressureChs, pressureCh)
		cwpoolCh := make(chan int, 1000)
		cwpoolChs = append(cwpoolChs, cwpoolCh)
		sd := rand.Int63()
		go func(s int64) {
			err := runExperiment(*srcSize, *differenceSize, *reverseDifferenceSize, *timeoutDuration, *timeoutCounter, *refillTransaction, ch, degreeCh, pressureCh, cwpoolCh, *degreeDistString, s)
			if err != nil {
				fmt.Println(err)
			}
		}(sd)
	}

	var f *os.File
	var degreeF *os.File
	var pressureF *os.File
	var cwpoolF *os.File
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
		fmt.Fprintf(f, "# num decoded     symbols rcvd\n")

		degreeF, err = os.Create(*outputPrefix+"-codeword-degree-dist.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Fprintf(degreeF, "# codeword degree     count\n")

		pressureF, err = os.Create(*outputPrefix+"-ntx-unique-to-p1.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Fprintf(pressureF, "# iteration     unique to P1\n")
		defer pressureF.Close()

		cwpoolF, err = os.Create(*outputPrefix+"-p2-codeword-pool.dat")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Fprintf(cwpoolF, "# iteration     P2 unreleased cw\n")
		defer cwpoolF.Close()
	}

	// monitor and dump to files
	wg := &sync.WaitGroup{}
	// for each tx idx, range over res channels to collect data and dump to file
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(degreeCh)	// we are sure that after all res channels close, no gorountine will write to degreeCh
		dch := make(chan int, 1000)
		idx := 0
		go collectAverage(chs, dch)
		for d := range dch {
			if f != nil {
				fmt.Fprintf(f, "%v        %v\n", idx, d)
			}
			if !*noTermOut {
				fmt.Printf("Iteration=%v, transactions=%v\n", d, idx)
			}
			idx += 1
		}
	}()
	// collect and dump codeword degree distribution
	if degreeF != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collectDumpHistogram(degreeF, degreeCh)
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

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			fmt.Println("could not create memory profile: ", err)
			os.Exit(1)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Println("could not write memory profile: ", err)
			os.Exit(1)
		}
	}

	wg.Wait()
	return
}

func runExperiment(s, d, r, tout, tcnt int, refill string, res, degree, diff, cwpool chan int, dist string, seed int64) error {
	defer close(res)	// close when the experiment ends
	defer close(diff)
	defer close(cwpool)
	rng := rand.New(rand.NewSource(seed))
	dist1, err := NewDistribution(rng, dist, d+r)
	if err != nil {
		return err
	}
	pacer1, err := NewTransactionPacer(rng, refill)
	if err != nil {
		return err
	}
	p1, err := newNode(nil, 0, s, dist1, rng, pacer1)
	if err != nil {
		return err
	}
	// TODO: d+r is not a good estimation anymore with refill and potentially empty starting sets
	dist2, err := NewDistribution(rng, dist, d+r)
	if err != nil {
		return err
	}
	pacer2, err := NewTransactionPacer(rng, refill)
	if err != nil {
		return err
	}
	p2, err := newNode(p1.TransactionPool, s-d, r, dist2, rng, pacer2)
	if err != nil {
		return err
	}

	res <- 0 // at iteration 0, we have decoded 0 transactions
	// start sending codewords from p1 to p2
	// prepare the counters
	i := 0					// iteration counter
	lastAct := 0				// last iteration where there's any progress
	received := make([]int, 2)		// transaction pool size as of the end of prev iter
	received[0] = len(p1.Transactions)
	received[1] = len(p2.Transactions)
	decoded := make([]int, 2)		// num transactions decoded
	decoded[0] = 0
	decoded[1] = 0
	unique := make([]int, 2)		// num transactions undecoded by the other end
	unique[0] = d
	unique[1] = r
	for ;; {
		i += 1
		c1 := p1.produceCodeword()
		c2 := p2.produceCodeword()
		degree <- c1.Counter
		p2.InputCodeword(c1)
		p2.TryDecode()
		p1.InputCodeword(c2)
		p1.TryDecode()
		for cnt := 0; cnt < len(p2.Transactions) - received[1]; cnt++ {
			res <- i
			lastAct = i
			decoded[1] += 1
			unique[0] -= 1
			if tcnt != 0 && tcnt <= decoded[1] {
				return nil
			}
		}
		for cnt := 0; cnt < len(p1.Transactions) - received[0]; cnt++ {
			lastAct = i
			decoded[0] += 1
			unique[1] -= 1
		}
		if i - lastAct > tout {
			return nil
		}
		// add transactions to pools
		nadd := p1.pacer.tick(unique[0])
		for cnt := 0; cnt < nadd; cnt++ {
			t := p1.getRandomTransaction()
			p1.AddTransaction(t)
			unique[0] += 1
			/*
			if rand.Float64() < 0.8 {
				p2.AddTransaction(t)
			} else {
				unique += 1
			}
			*/
		}
		nadd = p2.pacer.tick(unique[1])
		for cnt := 0; cnt < nadd; cnt++ {
			t := p2.getRandomTransaction()
			p2.AddTransaction(t)
			unique[1] += 1
			/*
			if rand.Float64() < 0.8 {
				p1.AddTransaction(t)
			}
			*/
		}
		received[0] = len(p1.Transactions)
		received[1] = len(p2.Transactions)
		diff <- unique[0]
		cwpool <- len(p2.Codewords)
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
	TimeoutCounter int
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

func collectDumpHistogram(f *os.File, ch chan int) {
	hist := make(map[int]int)
	maxd := 0
	tot := 0
	for d := range ch {
		hist[d] += 1
		if maxd < d {
			maxd = d
		}
		tot += 1
	}
	for i:=0; i <=maxd; i++ {
		if val, there := hist[i]; there {
			fmt.Fprintf(f, "%v         %v\n", i, float64(val)/float64(tot))
		}
	}
}

func collectAverage(chs []chan int, out chan int) {
	defer close(out)
	for ;; {
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
			out <- d/len(chs)
		}
	}
}
