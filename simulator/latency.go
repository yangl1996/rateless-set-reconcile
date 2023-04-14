package main

import (
	_ "embed"
	"fmt"
	"strings"
	"time"
	"github.com/yangl1996/rateless-set-reconcile/des"
)

//go:embed city-prop-delay.csv
var citiesTopologyLatencyData string

type pairwiseLatency struct {
	l [][]time.Duration
	m map[des.Module]int
	nextFree int
}

func (p *pairwiseLatency) register(m des.Module) {
	p.m[m] = p.nextFree
	p.nextFree += 1
}

func (p *pairwiseLatency) PropagationDelay(from des.Module, to des.Module) time.Duration {
	return p.l[p.m[from]][p.m[to]]
}

func loadCitiesTopology() *pairwiseLatency {
	res := make([][]time.Duration, 250)
	for i := range res {
		res[i] = make([]time.Duration, 250)
	}
	entries := strings.Split(citiesTopologyLatencyData, "\n")
	for _, e := range entries {
		var from, to int
		var l float64
		n, _ := fmt.Sscanf(e, "%d,%d,%f,", &from, &to, &l)
		if n == 3 {
			d := time.Duration(l * float64(time.Millisecond))
			res[from][to] = d
			res[to][from] = d
		}
	}
	return &pairwiseLatency{
		l: res,
		m: make(map[des.Module]int),
		nextFree: 0,
	}
}
