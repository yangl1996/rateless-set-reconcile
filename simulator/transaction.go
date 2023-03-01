package main

import (
	"encoding/binary"
	"github.com/yangl1996/rateless-set-reconcile/lt"
	"github.com/DataDog/sketches-go/ddsketch"
	"time"
)

type transaction struct {
	idx uint64
	ts time.Duration
}

func (d transaction) XOR(t2 transaction) transaction {
	return transaction{d.idx ^ t2.idx, d.ts ^ t2.ts}
}

func (d transaction) Hash() []byte {
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, d.idx)
	return res
}

type transactionGenerator struct {
	next uint64
}

func newTransactionGenerator() *transactionGenerator {
	return &transactionGenerator{}
}

func (t *transactionGenerator) generate(at time.Duration) lt.Transaction[transaction] {
	tx := transaction{t.next, at}
	t.next += 1
	return lt.NewTransaction[transaction](tx)
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
	latency := tp.Seconds() - tx.ts.Seconds()
	t.sketch.Add(latency)
}

func (t *transactionLatencySketch) getQuantiles(q []float64) []float64 {
	res, err := t.sketch.GetValuesAtQuantiles(q)
	if err != nil {
		panic(err)
	}
	return res
}
