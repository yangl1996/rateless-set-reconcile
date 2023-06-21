package riblt

import (
	"github.com/dchest/siphash"
	//"math/rand"
)

type pendingSymbol[T Symbol[T]] struct {
	CodedSymbol[T]
	salt0, salt1 uint64
	threshold uint64
}

type Decoder[T Symbol[T]] struct {
	pending []PendingSymbol[T]
	window []HashedSymbol[T]
	removed []HashedSymbol[T]
	added []HashedSymbol[T]
}

func (d *Decoder[T]) AddCodedSymbol(c CodedSymbol[T], salt0, salt1, threshold uint64) {
	// scan through decoded symbols to peel off matching ones
	for _, v := range e.window {
		sh := siphash.Hash(salt0, salt1, v.hash)
		if sh < threshold {
			c.symbol = c.symbol.XOR(v.symbol)
			c.count -= 1
			c.checksum ^= sh
		}
	}
	switch c.count {
	case 0:
		if c.checksum == 0 {
			// new codeword is cleared without yielding any new symbol
			return nil
		}
	case 1:

	case -1:
	}
	// default
}

/*
type Encoder[T Symbol[T]] struct {
	window     []HashedSymbol[T]
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
	Threshold(idx uint64) uint64
}

type SynchronizedEncoder[T Symbol[T]] struct {
	r *rand.Rand	// FIXME: is it safe to assume the RNG implementation is platform/OS/arch agnostic? Probably better to use an explicit algorithm.
	encoder *Encoder[T]
	degseq DegreeSequence
	count uint64
}

func (e *SynchronizedEncoder[T]) ProduceNextCodedSymbol() CodedSymbol[T] {
	salt0 := e.r.Uint64()
	salt1 := e.r.Uint64()
	threshold := e.degseq.Threshold(e.count)
	s := e.encoder.ProduceCodedSymbol(salt0, salt1, threshold)
	e.count += 1
	return s
}

func (e *SynchronizedEncoder[T]) Reset() {
	e.encoder.Reset()
	e.count = 0
}
*/
