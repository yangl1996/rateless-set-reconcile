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
	pending []pendingSymbol[T]
	window []HashedSymbol[T]
	removed []HashedSymbol[T]
	added []HashedSymbol[T]
}

func (d *Decoder[T]) tryDecode() {
	for ;; {
		acted := false
		for i, p := range d.pending {
			switch p.count {
			case 0:
				if p.checksum == 0 {
					d.pending[i] = d.pending[len(d.pending)-1]
					d.pending = d.pending[:len(d.pending)-1]
					acted = true
				}
			case 1:
				s := NewHashedSymbol[T](p.sum)
				if siphash.Hash(p.salt0, p.salt1, s.hash) == p.checksum {
					d.pending[i] = d.pending[len(d.pending)-1]
					d.pending = d.pending[:len(d.pending)-1]
					for j := range d.pending {
						sh := siphash.Hash(d.pending[j].salt0, d.pending[j].salt1, s.hash)
						if sh < d.pending[j].threshold {
							d.pending[j].sum = d.pending[j].sum.XOR(s.symbol)
							d.pending[j].count -= 1
							d.pending[j].checksum ^= sh
						}
					}
					d.window = append(d.window, s)
					d.added = append(d.added, s)
					acted = true
				}
			case -1:
				s := NewHashedSymbol[T](p.sum)
				if siphash.Hash(p.salt0, p.salt1, s.hash) == p.checksum {
					d.pending[i] = d.pending[len(d.pending)-1]
					d.pending = d.pending[:len(d.pending)-1]
					for j := range d.pending {
						sh := siphash.Hash(d.pending[j].salt0, d.pending[j].salt1, s.hash)
						if sh < d.pending[j].threshold {
							d.pending[j].sum = d.pending[j].sum.XOR(s.symbol)
							d.pending[j].count += 1
							d.pending[j].checksum ^= sh
						}
					}
					d.removed = append(d.removed, s)
					// TODO: remove from window
					acted = true
				}
			}
			if acted {
				break
			}
		}
		if !acted {
			break
		}
	}
}

func (d *Decoder[T]) AddCodedSymbol(c CodedSymbol[T], salt0, salt1, threshold uint64) {
	// scan through decoded symbols to peel off matching ones
	for _, v := range d.window {
		sh := siphash.Hash(salt0, salt1, v.hash)
		if sh < threshold {
			c.sum = c.sum.XOR(v.symbol)
			c.count -= 1
			c.checksum ^= sh
		}
	}
	p := pendingSymbol[T]{c, salt0, salt1, threshold}
	d.pending = append(d.pending, p)
	d.tryDecode()
	return
}

func (d *Decoder[T]) Reset() {
	if len(d.pending) != 0 {
		d.pending = d.pending[:0]
	}
	if len(d.window) != 0 {
		d.window = d.window[:0]
	}
	if len(d.removed) != 0 {
		d.removed = d.removed[:0]
	}
	if len(d.added) != 0 {
		d.added = d.added[:0]
	}
}

/*
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
