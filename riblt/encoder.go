package riblt

import (
	"github.com/dchest/siphash"
	"math/rand"
)
// TODO: can we use XOR w/ random number to replace siphash with random key?

type CodedSymbol[T Symbol[T]] struct {
	sum T
	count int64
	checksum uint64
}

type Encoder[T Symbol[T]] struct {
	window     []HashedSymbol[T]
}

func (e *Encoder[T]) ProduceCodedSymbol(salt0, salt1, threshold uint64) CodedSymbol[T] {
	c := CodedSymbol[T]{}

	for _, v := range e.window {
		sh := siphash.Hash(salt0, salt1, v.hash)
		if sh < threshold {
			c.sum = c.sum.XOR(v.symbol)
			c.count += 1
			c.checksum ^= sh
		}
	}

	return c
}

func (e *Encoder[T]) Reset() {
	if len(e.window) != 0 {
		e.window = e.window[:0]
	}
}

func (e *Encoder[T]) AddSymbol(t HashedSymbol[T]) {
	e.window = append(e.window, t)
}

type DegreeSequence interface {
	NextThreshold() uint64
	Reset()
}

type SynchronizedEncoder[T Symbol[T]] struct {
	r *rand.Rand	// FIXME: is it safe to assume the RNG implementation is platform/OS/arch agnostic? Probably better to use an explicit algorithm.
	encoder *Encoder[T]
	degseq DegreeSequence
}

func (e *SynchronizedEncoder[T]) ProduceNextCodedSymbol() CodedSymbol[T] {
	salt0 := e.r.Uint64()
	salt1 := e.r.Uint64()
	threshold := e.degseq.NextThreshold()
	s := e.encoder.ProduceCodedSymbol(salt0, salt1, threshold)
	return s
}

func (e *SynchronizedEncoder[T]) Reset() {
	e.encoder.Reset()
	e.degseq.Reset()
}

func (e *SynchronizedEncoder[T]) AddSymbol(t HashedSymbol[T]) {
	e.encoder.AddSymbol(t)
}
