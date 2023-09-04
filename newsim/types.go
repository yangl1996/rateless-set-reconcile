package main

import (
	"encoding/binary"
	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/dchest/siphash"
	"math"
	"time"
)

type degseq struct {
	count int
}

func (t *degseq) Reset() {
	t.count = 0
}

func (t *degseq) NextThreshold() uint64 {
	var th uint64
	th = math.MaxUint64
	if t.count != 0 {
		th = uint64(float64(th) / (1 + float64(t.count)/2))
	}
	t.count += 1
	return th
}

type transaction struct {
	idx uint64
	ts  time.Duration
}

func (d transaction) XOR(t2 transaction) transaction {
	return transaction{d.idx ^ t2.idx, d.ts ^ t2.ts}
}

func (d transaction) Hash() uint64 {
	var serialized [8]byte
	binary.LittleEndian.PutUint64(serialized[0:8], d.idx)
	return siphash.Hash(567, 890, serialized[:])
}

type transactionGenerator struct {
	last uint64
}

func (t *transactionGenerator) generate(at time.Duration) transaction {
	t.last += 1
	return transaction{t.last, at}
}

type distributionSketch struct {
	sketch *ddsketch.DDSketch
	warmup time.Duration
}

func newDistributionSketch(warmup time.Duration) *distributionSketch {
	sketch, err := ddsketch.NewDefaultDDSketch(0.01)
	if err != nil {
		panic(err)
	}
	return &distributionSketch{sketch, warmup}
}

func (t *distributionSketch) recordRaw(data float64, tp time.Duration) {
	if t == nil {
		return
	}
	if t.warmup > tp {
		return
	}
	t.sketch.Add(data)
}

func (t *distributionSketch) recordTxLatency(tx transaction, tp time.Duration) {
	if t == nil {
		return
	}
	if t.warmup > tp {
		return
	}
	latency := tp.Seconds() - tx.ts.Seconds()
	t.sketch.Add(latency)
}

func (t *distributionSketch) getQuantiles(q []float64) []float64 {
	res, err := t.sketch.GetValuesAtQuantiles(q)
	if err != nil {
		panic(err)
	}
	return res
}
