package main

import (
	"os"
	"io"
	"encoding/json"
)


type ExperimentConfig struct {
        MirrorProb            float64	// prob that a tx arriving at a node is mirrored to all other nodes
        Seed                  int64
        TimeoutDuration       int	// num of no-decode rounds to wait before exit
        TimeoutCounter        int	// num of transactions to decode on one node
        DegreeDist      string
        LookbackTime          uint64
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
