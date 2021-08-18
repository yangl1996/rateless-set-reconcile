package main

import (
	"os"
	"io"
	"encoding/json"
	"flag"
)

// debug features
var runtimetrace = flag.String("trace", "", "write trace to `file`")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
var outputPrefix = flag.String("out", "out", "output data path prefix, no output if empty")
// exp parameters
var runs = flag.Int("p", 1, "number of parallel runs")
var commonTx = flag.Int("s", 0, "number of initial common tx at each node")
var uniqueTx = flag.Int("x", 0, "number of initial unique tx at each node")
var mirrorProb = flag.Float64("m", 0, "probability that a new transaction arriving at a node appears at all nodes")
var seed = flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
var refillTransaction = flag.String("f", "p(0.7)", "transaction arrival pattern at each node: c(r) for uniform arrival at rate r per codeword, p(r) for poisson arrival at rate r, empty string to disable")
var timeoutDuration = flag.Int("t", 500, "stop the experiment if no new transaction is decoded after this amount of codewords")
var timeoutCounter = flag.Int("tc", 0, "number of transactions to decode before stopping, 0 for unlimited")
var degreeDistString = flag.String("d", "u(0.01)", "distribution of parity check degrees: rs(k,c,delta) for robust soliton with parameters k, c, and delta, s(k) for soliton with parameter k where k is usually the length of the encoded data, u(f) for uniform with fraction=f, b(f1, f2, p1) for bimodal with fraction=f1 with probability p1, and fraction=f2 with probability=1-p1")
var lookbackTime = flag.Uint64("l", 0, "lookback timespan of codewords, 0 for infinity")
var readConfig = flag.String("c", "", "read config from `file`; the config will be overwritten by parameters passed through command line")

func updateConfig(cfg *ExperimentConfig, f *flag.Flag) {
	switch f.Name {
	case "p":
		cfg.ParallelRuns = *runs
	case "s":
		cfg.Topology.InitialCommonTx = *commonTx
	case "x":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].InitialUniqueTx = *uniqueTx
		}
	case "m":
		cfg.MirrorProb = *mirrorProb
	case "seed":
		cfg.Seed = *seed
	case "f":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].TxArrivePattern = *refillTransaction
		}
	case "t":
		cfg.TimeoutDuration = *timeoutDuration
	case "tc":
		cfg.TimeoutCounter = *timeoutCounter
	case "d":
		cfg.DegreeDist = *degreeDistString
	case "l":
		cfg.LookbackTime = *lookbackTime
	}
}

func getConfig() (ExperimentConfig, error) {
	var err error
	var cfg ExperimentConfig
	// first see if we need to read from config file
	if *readConfig != "" {
		cfg, err = readConfigFile(*readConfig)
		if err != nil {
			return cfg, err
		}
		flag.Visit(func (f *flag.Flag) {
			updateConfig(&cfg, f)
		})
	} else {
		cfg = ExperimentConfig {
			MirrorProb: *mirrorProb,
			Seed: *seed,
			TimeoutDuration: *timeoutDuration,
			TimeoutCounter: *timeoutCounter,
			DegreeDist: *degreeDistString,
			LookbackTime: *lookbackTime,
			ParallelRuns: *runs,
			Topology: Topology {
				Servers: []Server {
					Server {
						Name: "node1",
						InitialUniqueTx: *uniqueTx,
						TxArrivePattern: *refillTransaction,
					},
					Server {
						Name: "node2",
						InitialUniqueTx: *uniqueTx,
						TxArrivePattern: *refillTransaction,
					},
				},
				InitialCommonTx: *commonTx,
				Connections: []Connection {
					Connection {"node1", "node2"},
				},
			},
		}
	}
	// validate the dist and pacer strings
	_, err = NewDistribution(nil, cfg.DegreeDist, 10000)
        if err != nil {
		return cfg, err
        }
	for _, s := range cfg.Topology.Servers {
		_, err = NewTransactionPacer(nil, s.TxArrivePattern)
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

type ExperimentConfig struct {
        MirrorProb            float64	// prob that a tx arriving at a node is mirrored to all other nodes
        Seed                  int64
        TimeoutDuration       int	// num of no-decode rounds to wait before exit
        TimeoutCounter        int	// num of transactions to decode on one node
        DegreeDist      string
        LookbackTime          uint64
	ParallelRuns int
	Topology
}

type Topology struct {
	Servers []Server
	InitialCommonTx int
	Connections []Connection
}

type Server struct {
	Name string
	InitialUniqueTx int
	TxArrivePattern string
}

type Connection struct {
	Car string
	Cdr string
}

func readConfigFile(path string) (ExperimentConfig, error) {
	cf := ExperimentConfig{}
	f, err := os.Open(path)
	if err != nil {
		return cf, err
	}
	defer f.Close()
	fc, err := io.ReadAll(f)
	if err != nil {
		return cf, err
	}
	err = json.Unmarshal(fc, &cf)
	return cf, err
}

func writeConfigFile(path string, cf *ExperimentConfig) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.MarshalIndent(cf, "", " ")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
