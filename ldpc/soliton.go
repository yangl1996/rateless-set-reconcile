package ldpc

import (
	"math/rand"
	"math/big"
	"sort"
)

// Soliton implements the Soliton distribution. It has a single parameter, K,
// and the probability density function P is
//  P(1)=1/K
//  P(x)=1/x(x-1) for x=2 to K
type Soliton struct {
	k uint64
	splits []float64	// the entire range of [0, 1) is cut into k pieces with k-1 splits

}

/*
// NewRobustSoliton creates a "robust" soliton distribution. delta controls the decoding
// error probability, and the expected ripple size is c*log(k/delta)sqrt(k).
func NewRobustSoliton(k uint64, c, delta float64) *Soliton {

}
*/

// rho implements the rho(x) function in soliton distribution for x>1.
func rho(i uint64) *big.Float {
	one := new(big.Float).SetFloat64(1.0)
	t1 := new(big.Float).SetUint64(i)
	t2 := new(big.Float).SetUint64(i-1)
	div := new(big.Float).Mul(t1, t2)
	return new(big.Float).Quo(one, div)
}

// rho implements the rho(x) function in soliton distribution with parameter k for x=1.
func rhoOne(k uint64) *big.Float {
	one := new(big.Float).SetFloat64(1.0)
	div := new(big.Float).SetUint64(k)
	return new(big.Float).Quo(one, div)
}

func NewSoliton(k uint64) *Soliton {
	var s []float64
	last := new(big.Float).SetUint64(0)
	var i uint64
	for i=1; i<k; i++ {	// we only do 1 to k-1 (incl.) because we only need k-1 splits
		var p *big.Float
		if i == 1 {
			p = rhoOne(k)
		} else {
			p = rho(i)
		}
		last = last.Add(last, p)
		rounded, _ := last.Float64()
		s = append(s, rounded)
	}
	s = append(s, 1.0)
	return &Soliton{k, s}
}

// Uint64 returns a value drawn from the given Soliton distribution.
func (s *Soliton) Uint64() uint64 {
	r := rand.Float64()
	idx := sort.SearchFloat64s(s.splits, r)
	if uint64(idx) >= s.k {
		panic("r should never be larger than the last item in s")
	}
	return uint64(idx+1)
}
