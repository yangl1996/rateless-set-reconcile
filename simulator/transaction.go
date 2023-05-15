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
	sketch *ddsketch.DDSketch
	warmup time.Duration
}

func newTransactionLatencySketch(warmup time.Duration) *transactionLatencySketch {
	sketch, err := ddsketch.NewDefaultDDSketch(0.01)
	if err != nil {
		panic(err)
	}
	return &transactionLatencySketch{sketch, warmup}
}

func (t *transactionLatencySketch) recordRaw(data float64, tp time.Duration) {
	if t == nil {
		return
	}
	if t.warmup > tp {
		return
	}
	t.sketch.Add(data)
}

func (t *transactionLatencySketch) record(tx transaction, tp time.Duration) {
	if t == nil {
		return
	}
	if t.warmup > tp {
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
