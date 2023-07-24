package riblt

import (
	"math/rand"
)

type pendingSymbol[T Symbol[T]] struct {
	CodedSymbol[T]
	salt uint64
	threshold uint64
	pending bool
	dirty bool
}

type Decoder[T Symbol[T]] struct {
	cs []pendingSymbol[T]	// coded symbols received so far
	local []HashedSymbol[T]
	window []HashedSymbol[T]	// set of the symbols that the decoder already has
	remote []HashedSymbol[T]
	pending []int	// indices of the coded symbols in cs that are not yet pure (decoded)
	dirty []int		// indices of the coded symbols in cs that have been operated on (peeled) but not checked for pureness
	pure []int
}

func (d *Decoder[T]) Decoded() bool {
	return len(d.pending) == 0
}

func (d *Decoder[T]) Local() []HashedSymbol[T] {
	return d.local
}

func (d *Decoder[T]) Remote() []HashedSymbol[T] {
	return d.remote
}

func (d *Decoder[T]) AddSymbol(s T) {
	th := HashedSymbol[T]{s, s.Hash()}
	d.AddHashedSymbol(th)
}

// TODO: update the definition of dirty to cover only codewords that have some chance to get decoded.
func (d *Decoder[T]) AddHashedSymbol(s HashedSymbol[T]) {
	d.window = append(d.window, s)
	for idx := range d.remote {
		if s.Hash == d.remote[idx].Hash {
			l := len(d.remote)
			d.remote[idx] = d.remote[l-1]
			d.remote = d.remote[:l-1]
			return
		}
	}
	for _, j := range d.pure {
		if d.cs[j].salt * s.Hash < d.cs[j].threshold {
			d.local = append(d.local, s)
			return
		}
	}
	pidx := 0
	ptot := len(d.pending)
	for pidx < ptot {
		j := d.pending[pidx]
		if d.cs[j].salt * s.Hash < d.cs[j].threshold {
			d.cs[j].sum = d.cs[j].sum.XOR(s.Symbol)
			d.cs[j].count -= 1
			d.cs[j].checksum ^= s.Hash
			if d.cs[j].count == 0 && d.cs[j].checksum == 0 {
				// d.cs[j] is now pure, remove it from pending list
				d.cs[j].dirty = false	// force it to be undirty so that we never look at it again
				d.cs[j].pending = false
				d.pending[pidx] = d.pending[ptot-1]
				d.pending = d.pending[:ptot-1]
				ptot -= 1
				d.pure = append(d.pure, j)
			} else {
				// d.cs[j] is now dirty
				if !d.cs[j].dirty {
					d.cs[j].dirty = true
					d.dirty = append(d.dirty, j)
				}
				pidx += 1
			}
		}
	}
}

func (d *Decoder[T]) AddCodedSymbol(c CodedSymbol[T], salt, threshold uint64) {
	// scan through decoded symbols to peel off matching ones
	for _, v := range d.window {
		if salt * v.Hash < threshold {
			c.sum = c.sum.XOR(v.Symbol)
			c.count -= 1
			c.checksum ^= v.Hash
		}
	}
	for _, v := range d.remote {
		if salt * v.Hash < threshold {
			c.sum = c.sum.XOR(v.Symbol)
			c.count -= 1
			c.checksum ^= v.Hash
		}
	}
	for _, v := range d.local {
		if salt * v.Hash < threshold {
			c.sum = c.sum.XOR(v.Symbol)
			c.count += 1
			c.checksum ^= v.Hash
		}
	}
	if c.count == 0 && c.checksum == 0 {
		// still insert the codeword in case a symbol added later causes it to become dirty
		p := pendingSymbol[T]{c, salt, threshold, false, false}
		d.cs = append(d.cs, p)
		d.pure = append(d.pure, len(d.cs)-1)
		return
	} else {
		p := pendingSymbol[T]{c, salt, threshold, true, true}
		d.cs = append(d.cs, p)
		d.dirty = append(d.dirty, len(d.cs)-1)
		d.pending = append(d.pending, len(d.cs)-1)
		return
	}
}

func (d *Decoder[T]) TryDecode() {
	// Go through all dirty coded symbols to see if any can be decoded.
	didx := 0
	dtot := len(d.dirty)
	for didx < dtot {
		i := d.dirty[didx]	// index of the coded symbol we currently examine
		didx += 1
		p := d.cs[i]
		if !p.dirty {
			// This (coded symbol being in the dirty list but not marked
			// dirty) is possible. For example, a symbol may first be marked
			// dirty and appended to the list, and then become pure and marked
			// as undirty.
			continue
		}
		d.cs[i].dirty = false
		switch p.count {
		case 1:
			h := p.sum.Hash()
			if h == p.checksum {
				// p.sum is now a symbol that only the peer has
				pidx := 0
				ptot := len(d.pending)
				for pidx < ptot {
					j := d.pending[pidx]
					if i == j {
						d.pending[pidx] = d.pending[ptot-1]
						d.pending = d.pending[:ptot-1]
						ptot -= 1
					} else if d.cs[j].salt * h < d.cs[j].threshold {
						d.cs[j].sum = d.cs[j].sum.XOR(p.sum)
						d.cs[j].count -= 1
						d.cs[j].checksum ^= h
						if d.cs[j].count == 0 && d.cs[j].checksum == 0 {
							// d.cs[j] is now pure, remove it from pending list
							d.cs[j].dirty = false	// force it to be undirty so that we never look at it again
							d.cs[j].pending = false 
							d.pending[pidx] = d.pending[ptot-1]
							d.pending = d.pending[:ptot-1]
							ptot -= 1
							d.pure = append(d.pure, j)
						} else {
							// d.cs[j] is now dirty
							if !d.cs[j].dirty {
								d.cs[j].dirty = true
								d.dirty = append(d.dirty, j)
								dtot += 1
							}
							pidx += 1
						}
					} else {
						pidx += 1
					}
				}
				s := HashedSymbol[T]{p.sum, h}
				d.remote = append(d.remote, s)
				d.cs[i].CodedSymbol = CodedSymbol[T]{}
				d.cs[i].pending = false 
				d.pure = append(d.pure, i)
			}
		case -1:
			h := p.sum.Hash()
			if h == p.checksum {
				// p.sum is now a symbol that only we have
				pidx := 0
				ptot := len(d.pending)
				for pidx < ptot {
					j := d.pending[pidx]
					if i == j {
						d.pending[pidx] = d.pending[ptot-1]
						d.pending = d.pending[:ptot-1]
						ptot -= 1
					} else if d.cs[j].salt * h < d.cs[j].threshold {
						d.cs[j].sum = d.cs[j].sum.XOR(p.sum)
						d.cs[j].count += 1
						d.cs[j].checksum ^= h
						if d.cs[j].count == 0 && d.cs[j].checksum == 0 {
							// d.cs[j] is now pure, remove it from pending list
							d.cs[j].dirty = false	// force it to be undirty so that we never look at it again
							d.cs[j].pending = false 
							d.pending[pidx] = d.pending[ptot-1]
							d.pending = d.pending[:ptot-1]
							ptot -= 1
							d.pure = append(d.pure, j)
						} else {
							// d.cs[j] is now dirty
							if !d.cs[j].dirty {
								d.cs[j].dirty = true
								d.dirty = append(d.dirty, j)
								dtot += 1
							}
							pidx += 1
						}
					} else {
						pidx += 1
					}
				}
				s := HashedSymbol[T]{p.sum, h}
				d.local = append(d.local, s)
				d.cs[i].CodedSymbol = CodedSymbol[T]{}
				d.cs[i].pending = false 
				d.pure = append(d.pure, i)
			}
		}
	}
	d.dirty = d.dirty[:0]
}

func (d *Decoder[T]) Reset() {
	if len(d.pending) != 0 {
		d.pending = d.pending[:0]
	}
	if len(d.window) != 0 {
		d.window = d.window[:0]
	}
	if len(d.local) != 0 {
		d.local = d.local[:0]
	}
	if len(d.remote) != 0 {
		d.remote = d.remote[:0]
	}
	if len(d.dirty) != 0 {
		d.dirty = d.dirty[:0]
	}
	if len(d.cs) != 0 {
		d.cs = d.cs[:0]
	}
	if len(d.pure) != 0 {
		d.pure = d.pure[:0]
	}
}

type SynchronizedDecoder[T Symbol[T]] struct {
	*rand.Rand	// FIXME: is it safe to assume the RNG implementation is platform/OS/arch agnostic? Probably better to use an explicit algorithm.
	*Decoder[T]
	DegreeSequence
}

func (d *SynchronizedDecoder[T]) AddNextCodedSymbol(c CodedSymbol[T]) {
	salt := d.Rand.Uint64()
	threshold := d.DegreeSequence.NextThreshold()
	d.Decoder.AddCodedSymbol(c, salt, threshold)
}

func (d *SynchronizedDecoder[T]) Reset() {
	d.Decoder.Reset()
	d.DegreeSequence.Reset()
}
