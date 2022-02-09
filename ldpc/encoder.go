package ldpc

import (
	"github.com/dchest/siphash"
	"hash"
	"math"
	"math/rand"
	"sync"
)

type DegreeDistribution interface {
	Uint64() uint64
}

type saltedTransaction struct {
	saltedHash uint32
	*Transaction
}

type Encoder struct {
	window     []saltedTransaction
	hasher     hash.Hash64
	degreeDist DegreeDistribution
	windowSize int
}

func NewEncoder(salt [SaltSize]byte, dist DegreeDistribution, ws int) *Encoder {
	p := &Encoder{
		hasher:     siphash.New(salt[:]),
		degreeDist: dist,
		windowSize: ws,
	}
	return p
}

func (e *Encoder) AddTransaction(t *Transaction) {
	e.hasher.Reset()
	e.hasher.Write(t.hash[:])
	hash := (uint32)(e.hasher.Sum64())
	tx := saltedTransaction{hash, t}
	e.window = append(e.window, tx)
	if len(e.window) > e.windowSize {
		diff := len(e.window) - e.windowSize
		e.window = e.window[diff:]
	}
}

func (e *Encoder) ProduceCodeword() *Codeword {
	deg := int(e.degreeDist.Uint64())
	return e.produceCodeword(deg)
}

type codewordBuilder []saltedTransaction

var codewordBuilderPool = sync.Pool{
	New: func() interface{} {
		return &codewordBuilder{}
	},
}

func (e *Encoder) produceCodeword(deg int) *Codeword {
	c := &Codeword{}
	if deg > len(e.window) {
		deg = len(e.window)
	}
	if deg == 0 {
		return c
	}
	c.Members = make([]uint32, deg)
	// reservoir sampling
	selected := codewordBuilderPool.Get().(*codewordBuilder)
	*selected = (*selected)[:0]
	for idx := 0; idx < deg; idx++ {
		*selected = append(*selected, e.window[idx])
	}
	d := float64(deg)
	var W float64
	W = math.Exp(math.Log(rand.Float64()) / d)
	midx := deg
	for midx < len(e.window) {
		midx += (int)(math.Floor(math.Log(rand.Float64())/math.Log(1.0-W))) + 1
		if midx < len(e.window) {
			(*selected)[rand.Intn(deg)] = e.window[midx]
			W = W * math.Exp(math.Log(rand.Float64())/d)
		}
	}
	for idx, item := range *selected {
		c.Members[idx] = item.saltedHash
		c.Symbol.XOR(&item.serialized)
		(*selected)[idx].Transaction = nil // set the ptr to nil so when selected is in the pool, it does not point to some transaction and cause it to remain in GC scope
	}
	codewordBuilderPool.Put(selected)
	return c
}
