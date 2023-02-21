package lt

import (
	"github.com/dchest/siphash"
	"hash"
	"math"
	"math/rand"
)

type Codeword[T TransactionData[T]] struct {
    symbol T
    members []uint32
}

const SaltSize = 16

type DegreeDistribution interface {
	Uint64() uint64
}

type saltedTransaction[T TransactionData[T]] struct {
	saltedHash uint32
	Transaction[T]
}

type Encoder[T TransactionData[T]] struct {
	window     []saltedTransaction[T]
	hasher     hash.Hash64
	degreeDist DegreeDistribution
	hashes map[uint32]struct{}	// transactions already in the window
	windowSize int

	codewordBuilder []saltedTransaction[T]
}

func NewEncoder[T TransactionData[T]](salt [SaltSize]byte, dist DegreeDistribution, ws int) *Encoder[T] {
	p := &Encoder[T]{
		hasher:     siphash.New(salt[:]),
		degreeDist: dist,
		windowSize: ws,
		hashes: make(map[uint32]struct{}),
	}
	return p
}

func (e *Encoder[T]) Reset(dist DegreeDistribution, ws int) {
	e.degreeDist = dist
	e.window = e.window[:0]
	e.windowSize = ws
	for k := range e.hashes {
		delete(e.hashes, k)
	}
}

func (e *Encoder[T]) AddTransaction(t Transaction[T]) bool {
	e.hasher.Reset()
	e.hasher.Write(t.hash[:])
	hash := (uint32)(e.hasher.Sum64())
	if _, there := e.hashes[hash]; there {
		// the transaction is already in the window
		return false
	}
	tx := saltedTransaction[T]{hash, t}
	e.window = append(e.window, tx)
	e.hashes[hash] = struct{}{}
	for len(e.window) > e.windowSize {
		delete(e.hashes, e.window[0].saltedHash)
		e.window = e.window[1:]
	}
	return true
}

func (e *Encoder[T]) ProduceCodeword() Codeword[T] {
	deg := int(e.degreeDist.Uint64())
	return e.produceCodeword(deg)
}

func (e *Encoder[T]) produceCodeword(deg int) Codeword[T] {
	c := Codeword[T]{}
	if deg > len(e.window) {
		deg = len(e.window)
	}
	if deg == 0 {
		panic("trying to produce codeword with degree zero")
	}
	c.members = make([]uint32, deg)

	// reservoir sampling
	e.codewordBuilder = e.codewordBuilder[:0]
	for idx := 0; idx < deg; idx++ {
		e.codewordBuilder = append(e.codewordBuilder, e.window[idx])
	}
	d := float64(deg)
	var W float64
	W = math.Exp(math.Log(rand.Float64()) / d)
	midx := deg
	for midx < len(e.window) {
		midx += (int)(math.Floor(math.Log(rand.Float64())/math.Log(1.0-W))) + 1
		if midx < len(e.window) {
			e.codewordBuilder[rand.Intn(deg)] = e.window[midx]
			W = W * math.Exp(math.Log(rand.Float64())/d)
		}
	}
	for idx, item := range e.codewordBuilder {
		c.members[idx] = item.saltedHash
		c.symbol = c.symbol.XOR(item.Transaction.data)
	}
	return c
}
