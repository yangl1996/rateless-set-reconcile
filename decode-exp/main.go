package main

import (
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
	flag.Parse()
	cfg, err := getConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if cfg.Seed == 0 {
		cfg.Seed = time.Now().UTC().UnixNano()
	}
	rand.Seed(cfg.Seed)

	// create output files
	var f *os.File
	var rippleF *os.File
	var pressureF *os.File
	var cwpoolF *os.File
	if *outputPrefix != "" {
		var err error
		err = writeConfigFile(*outputPrefix + ".cfg", &cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		f, err = os.Create(*outputPrefix + "-mean-iter-to-decode.dat")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
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
	for i := 0; i < cfg.ParallelRuns; i++ {
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
			err := runExperiment(cfg, ch, rippleCh, pressureCh, cwpoolCh)
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

func runExperiment(cfg ExperimentConfig, res, ripple, diff, cwpool chan int) error {
	if cfg.LookbackTime == 0 {
		cfg.LookbackTime = math.MaxUint64
	}
	rng := rand.New(rand.NewSource(cfg.Seed))
	var nodes []*node
	nodeName := make(map[string]*node)
	for nidx := range cfg.Topology.Servers {
		dt, err := NewDistribution(rng, cfg.DegreeDist, 10000)
		if err != nil {
			return err
		}
		pc, err := NewTransactionPacer(rng, cfg.Topology.Servers[nidx].TxArrivePattern)
		if err != nil {
			return err
		}
		p := newNode(dt, rng, pc, cfg.LookbackTime)
		nodeName[cfg.Topology.Servers[nidx].Name] = p
		nodes = append(nodes, p)
	}
	for _, cn := range cfg.Topology.Connections {
		nodeName[cn.Car].connectTo(nodeName[cn.Cdr])
	}

	p1tx := nodes[0].fillInitTransaction(nil, 0, cfg.Topology.InitialCommonTx + cfg.Topology.Servers[0].InitialUniqueTx)
	for nidx := 1; nidx < len(cfg.Topology.Servers); nidx++ {
		nodes[nidx].fillInitTransaction(p1tx, cfg.Topology.InitialCommonTx, cfg.Topology.Servers[nidx].InitialUniqueTx)
	}

	if res != nil {
		res <- 0 // at iteration 0, we have decoded 0 transactions
	}
	// start sending codewords
	// prepare the counters
	totalTx := 0	// all txs generated by all nodes
	i := 0 // iteration counter
	for {
		i += 1
		for nidx := range nodes {
			nodes[nidx].sendCodewords()
		}
		for nidx := range nodes {
			updated := nodes[nidx].tryDecode()
			if nidx == 0 {
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
			if int(nodes[nidx].Seq-nodes[nidx].lastAct) > cfg.TimeoutDuration {
				return StuckError{nidx}
			}
			if cfg.TimeoutCounter != 0 && cfg.TimeoutCounter <= nodes[nidx].decoded {
				return TransactionCountError{nidx}
			}
			// add transactions to pools
			nadd := nodes[nidx].transactionPacer.tick()
			for cnt := 0; cnt < nadd; cnt++ {
				t := nodes[nidx].getRandomTransaction()
				nodes[nidx].AddLocalTransaction(t)
				totalTx += 1
				if rng.Float64() < cfg.MirrorProb {
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
