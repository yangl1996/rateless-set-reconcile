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
var commonTx = flag.Int("s", 0, "add initial common tx to each node")
var initialTx = flag.Int("x", 0, "number of initial unique tx at each source")
var seed = flag.Int64("seed", 0, "seed to use for the RNG, 0 to seed with time")
var refillTransaction = flag.String("f", "p(0.7)", "transaction arrival pattern at each source: c(r) for uniform arrival at rate r per codeword, p(r) for poisson arrival at rate r, empty string to disable")
var timeoutDuration = flag.Int("t", 500, "stop the experiment if no new transaction is decoded after this amount of codewords at any node")
var timeoutCounter = flag.Int("tc", 0, "number of transactions to decode before stopping, 0 for unlimited")
var degreeDistString = flag.String("d", "u(0.01)", "distribution of parity check degrees: rs(k,c,delta,diff) for robust soliton with parameters k, c, delta, and estimated diff, s(k,diff) for soliton with parameter k and estimated diff, u(f) for uniform with fraction=f, b(f1, f2, p1) for bimodal with fraction=f1 with probability p1, and fraction=f2 with probability=1-p1")
var lookbackTime = flag.Uint64("l", 500, "lookback timespan of codewords")
var readConfig = flag.String("c", "", "read config from `file`; the config will be overwritten by parameters passed through command line")

func updateConfig(cfg *ExperimentConfig, f *flag.Flag) {
	switch f.Name {
	case "s":
		var ln []string
		for _, n := range cfg.Topology.Servers {
			ln = append(ln, n.Name)
		}
		cfg.Topology.Sources = append(cfg.Topology.Sources, Source{"common", "", *commonTx, ln})
	case "x":
		for sidx := range cfg.Topology.Sources {
			cfg.Topology.Sources[sidx].InitialTx= *initialTx
		}
	case "seed":
		cfg.Seed = *seed
	case "f":
		for sidx := range cfg.Topology.Sources {
			cfg.Topology.Sources[sidx].ArrivePattern = *refillTransaction
		}
	case "t":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].TimeoutDuration= *timeoutDuration
		}
	case "tc":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].TimeoutCounter= *timeoutCounter
		}
	case "d":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].DegreeDist= *degreeDistString
		}
	case "l":
		for sidx := range cfg.Topology.Servers {
			cfg.Topology.Servers[sidx].LookbackTime= *lookbackTime
		}
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
			Seed: *seed,
			Topology: Topology {
				Servers: []Server {
					Server {
						Name: "node1",
						TimeoutDuration: *timeoutDuration,
						TimeoutCounter: *timeoutCounter,
						DegreeDist: *degreeDistString,
						LookbackTime: *lookbackTime,
					},
					Server {
						Name: "node2",
						TimeoutDuration: *timeoutDuration,
						TimeoutCounter: *timeoutCounter,
						DegreeDist: *degreeDistString,
						LookbackTime: *lookbackTime,
					},
				},
				Connections: []Connection {
					Connection {"node1", "node2"},
				},
				Sources: []Source {
					Source {
						Name: "source1",
						ArrivePattern: *refillTransaction,
						InitialTx: *initialTx,
						Targets: []string{"node1"},
					},
					Source {
						Name: "source2",
						ArrivePattern: *refillTransaction,
						InitialTx: *initialTx,
						Targets: []string{"node2"},
					},
					Source {
						Name: "common",
						ArrivePattern: "",
						InitialTx: *commonTx,
						Targets: []string{"node1", "node2"},
					},
				},
			},
		}
	}
	return cfg, nil
}

type ExperimentConfig struct {
	Seed                  int64
	Topology
}

type Topology struct {
	Servers []Server
	Connections []Connection
	Sources []Source
}

type Server struct {
	Name string
	TimeoutDuration       int	// num of no-decode rounds to wait before exit
	TimeoutCounter        int	// num of transactions to decode on one node
	DegreeDist      string
	LookbackTime          uint64
}

type Source struct {
	Name string
	ArrivePattern string
	InitialTx int
	Targets []string
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
