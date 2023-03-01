package main

import (
	"encoding/binary"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/DataDog/sketches-go/ddsketch"
	"time"
)

type transaction uint64

func (d transaction) XOR(t2 transaction) transaction {
	return d ^ t2
}

func (d transaction) Hash() []byte {
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, uint64(d))
	return res
}

func (d transaction) Equals(t2 transaction) bool {
	return d == t2
}

type transactionGenerator struct {
	next transaction 
	ts   map[transaction]time.Duration
}

func newTransactionGenerator() *transactionGenerator {
	return &transactionGenerator{
		ts: make(map[transaction]time.Duration),
	}
}

func (t *transactionGenerator) generate(at time.Duration) lt.Transaction[transaction] {
	tx := t.next
	t.ts[tx] = at
	t.next += 1
	return lt.NewTransaction[transaction](tx)
}

func (t *transactionGenerator) timestamp(tx transaction) time.Duration {
	return t.ts[tx]
}

type transactionLatencySketch struct {
	nextAdd transaction
	pending map[transaction]struct{}
	sketch *ddsketch.DDSketch
}

func newTransactionLatencySketch() *transactionLatencySketch {
	sketch, err := ddsketch.NewDefaultDDSketch(0.01)
	if err != nil {
		panic(err)
	}
	return &transactionLatencySketch{
		pending: make(map[transaction]struct{}),
		sketch: sketch,
	}
}

func (t *transactionLatencySketch) record(tx transaction, tp time.Duration) {
	if t == nil {
		return
	}
	latency := tp.Seconds() - txgen.timestamp(tx).Seconds()
	t.sketch.Add(latency)
}

func (t *transactionLatencySketch) getQuantiles(q []float64) []float64 {
	res, err := t.sketch.GetValuesAtQuantiles(q)
	if err != nil {
		panic(err)
	}
	return res
}
