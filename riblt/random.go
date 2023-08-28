package riblt

import (
	"math"
)

type randomMapping struct {
	prng uint64		// PRNG state
	lastIdx uint64	// the last index the symbol was mapped to
}

const (
	minstd_m uint64 = 2147483647
	minstd_a uint64 = 16807
)

// degree sequence is 1/(1+idx/2)
func (s *randomMapping) nextIndex() uint64 {
	r := (minstd_a * s.prng) % minstd_m
	s.prng = r
	// m: minstd_m
	// x: steps to advance
	// r: random integer in [0, m)
	// j: lastIdx
	// r/m = x(2j+x+3)/((j+x+1)(j+x+2))
	// TODO: consider taking log on both sides?
	// x = (-(2j+3) + sqrt((m (2j+3)^2 - r)/(m-r)))/2
	// x = Ceil(x)
	rs := float64(s.lastIdx)*2 + 3	// rs = 2j+3
	ls := math.Sqrt((float64(minstd_m) * rs * rs - float64(r)) / float64(minstd_m - r))
	s.lastIdx += uint64(math.Ceil((ls - rs)/2))
	if s.lastIdx > minstd_m {
		panic("overflow")
	}
	return s.lastIdx
}
