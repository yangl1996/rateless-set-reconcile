package riblt

import (
	"math/rand"
)

const order = 18446744073709551557	// 2^64-59

// TODO: prove that multiplying with a random number (mod prime number?) can replace keyed hash.

type CodedSymbol[T Symbol[T]] struct {
	sum T
	count int64
	checksum uint64
}

type Encoder[T Symbol[T]] struct {
	window     []HashedSymbol[T]
}

func (e *Encoder[T]) ProduceCodedSymbol(salt, threshold uint64) CodedSymbol[T] {
	c := CodedSymbol[T]{}

	for _, v := range e.window {
		sh := salt * v.Hash
		if sh < threshold {
			c.sum = c.sum.XOR(v.Symbol)
			c.count += 1
			c.checksum ^= v.Hash
		}
	}

	return c
}

func (e *Encoder[T]) Reset() {
	if len(e.window) != 0 {
		e.window = e.window[:0]
	}
}

func (e *Encoder[T]) AddHashedSymbol(t HashedSymbol[T]) {
	e.window = append(e.window, t)
}

func (e *Encoder[T]) AddSymbol(t T) {
	th := HashedSymbol[T]{t, t.Hash()}
	e.window = append(e.window, th)
}

type DegreeSequence interface {
	NextThreshold() uint64
	Reset()
}

type SynchronizedEncoder[T Symbol[T]] struct {
	*rand.Rand	// FIXME: is it safe to assume the RNG implementation is platform/OS/arch agnostic? Probably better to use an explicit algorithm.
	*Encoder[T]
	DegreeSequence
}

func (e *SynchronizedEncoder[T]) ProduceNextCodedSymbol() CodedSymbol[T] {
	salt := e.Rand.Uint64()
	threshold := e.DegreeSequence.NextThreshold()
	s := e.Encoder.ProduceCodedSymbol(salt, threshold)
	return s
}

func (e *SynchronizedEncoder[T]) Reset() {
	e.Encoder.Reset()
	e.DegreeSequence.Reset()
}

