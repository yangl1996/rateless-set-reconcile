package ldpc

import (
	"github.com/dchest/siphash"
	"github.com/yangl1996/soliton"
	"hash"
	"math"
	"math/rand"
)

type saltedTransaction struct {
	saltedHash uint32
	*Transaction
}

type Encoder struct {
	window []saltedTransaction
    hasher hash.Hash64
	degreeDist *soliton.Soliton
	windowSize int
}

func NewEncoder(salt [SaltSize]byte, dist *soliton.Soliton, ws int) *Encoder {
    p := &Encoder{
        hasher: siphash.New(salt[:]),
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

func(e *Encoder)ProduceCodeword() *Codeword {
	deg := int(e.degreeDist.Uint64())
	return e.produceCodeword(deg)
}

func (e *Encoder) produceCodeword(deg int) *Codeword {
	c := &Codeword{}
	if deg > len(e.window) {
		deg = len(e.window)
	}
	if deg == 0 {
		return c
	}
	c.members = make([]uint32, deg)
	// reservoir sampling
	selected := make([]saltedTransaction, deg)
	for idx := range selected {
		selected[idx] = e.window[idx]
	}
	d := float64(deg)
	var W float64
	W = math.Exp(math.Log(rand.Float64()) / d)
	midx := deg
	for midx < len(e.window) {
		midx += (int)(math.Floor(math.Log(rand.Float64())/math.Log(1.0-W)))+1
		if midx < len(e.window) {
			selected[rand.Intn(deg)] = e.window[midx]
			W = W * math.Exp(math.Log(rand.Float64()) / d)
		}
	}
	for idx, item := range selected {
		c.members[idx] = item.saltedHash
		c.symbol.XOR(&item.serialized)
	}
	return c
}
