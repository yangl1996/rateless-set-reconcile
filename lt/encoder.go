package lt

import (
	"github.com/dchest/siphash"
	"hash"
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
	r *rand.Rand
	window     []saltedTransaction[T]
	hasher     hash.Hash64
	degreeDist DegreeDistribution
	hashes map[uint32]struct{}	// transactions already in the window
	windowSize int

	shuffleHistory []int
}

func NewEncoder[T TransactionData[T]](r *rand.Rand, salt [SaltSize]byte, dist DegreeDistribution, ws int) *Encoder[T] {
	p := &Encoder[T]{
		r: r,
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

	// sample without replacement
	n := len(e.window)
	// record shuffle history so that we don't destory the ordering of items in
	// the window
	e.shuffleHistory = e.shuffleHistory[:0]
	for i := 0; i < deg; i++ {
		// randomly shuffle with any item with idx i, i+1, ..., n-1, i.e.,
		// items not yet selected
		r := e.r.Intn(n-i)+i
		e.shuffleHistory = append(e.shuffleHistory, r)
		e.window[i], e.window[r] = e.window[r], e.window[i]
		// this is now the selected item, add it to codeword
		c.symbol = c.symbol.XOR(e.window[i].Transaction.data)
		c.members[i] = e.window[i].saltedHash
	}
	// revert the shuffling
	for i := deg-1; i >= 0; i-- {
		e.window[i], e.window[e.shuffleHistory[i]] = e.window[e.shuffleHistory[i]], e.window[i]
	}
	return c
}
