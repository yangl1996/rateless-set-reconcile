package riblt

import (
	"math/rand"
)

//const order = 18446744073709551557	// 2^64-59, largest prime that fits in 64 bits
//const order = 4294967291 // 2^32-5, so we can use 64-bit type to store values and perform multiplication without worrying about overflow
const order = 2147483647 // 2^31-1, so 31-bit arithmetics work out of the box


// TODO: prove that multiplying with a random number (mod prime number?) can replace keyed hash.
// TODO: if we want hash, salt, and their multiplication to be in F_p (p is prime) then the fastest and simpliest way (I think?) is to let p be the largest prime number smaller than 2^32, and perform normal 64-bit multiplication hash*salt (result guaranteed to be within 2^64) and then take modulo. However we will be forced to use 4-byte hashes and salts. I think it is fine (we can use the per-peer salt trick everyone else is using) but might be good to see if there is a way to stick to 8-byte salts and hashes. A very simple solution is for the encoder to send conflicting transactions as-is when detected; since it happens very rare and each pair of peers can use a secret hash key, an adversary cannot forge too many conflicts.
// TODO: replace siphash with xxhash (or whatever that supports native 4-byte output)
// TODO: we can show that the inclusion/not of any two transactions is pairwise-independent. Is it enough?

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

