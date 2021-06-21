package ldpc

import (
	"math/rand"
	"math/big"
	"math"
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

// NewRobustSoliton creates a "robust" soliton distribution. delta controls the decoding
// error probability, and the expected ripple size is c*log(k/delta)sqrt(k).
func NewRobustSoliton(k uint64, c, delta float64) *Soliton {
	p := make([]float64, k)
	tot := 0.0
	var i uint64
	for i = 1; i <= k; i++ {
		r, _ := rho(k, i).Float64()
		p[i-1] = r + tau(c, delta, k, i)
		tot += p[i-1]
	}
	var s []float64
	last := 0.0
	for i = 1; i < k; i++ {
		last += p[i-1]
		s = append(s, last/tot)
	}
	s = append(s, 1.0)
	return &Soliton{k, s}
}

// TODO: consider precision
func tau(c, delta float64, k, i uint64) float64 {
	r := ripple(c, delta, k)
	th := uint64(float64(k)/r)
	if i < th {	// 1 to k/R-1
		return r/(float64(i*k))
	} else if i == th {	// k/R
		return r * math.Log(r/delta) / float64(k)
	} else {	// k/R+1 to k
		return 0
	}
}

// ripple calculates the expected ripple size of a robust soliton distribution.
func ripple(c, delta float64, k uint64) float64 {
	kf := float64(k)
	return c * math.Log(kf / delta) * math.Sqrt(kf)
}

// rho implements the rho(i) function in soliton distribution.
func rho(k, i uint64) *big.Float {
	if i == 1 {
		one := new(big.Float).SetFloat64(1.0)
		div := new(big.Float).SetUint64(k)
		return new(big.Float).Quo(one, div)
	} else {
		one := new(big.Float).SetFloat64(1.0)
		t1 := new(big.Float).SetUint64(i)
		t2 := new(big.Float).SetUint64(i-1)
		div := new(big.Float).Mul(t1, t2)
		return new(big.Float).Quo(one, div)
	}
}

func NewSoliton(k uint64) *Soliton {
	var s []float64
	last := new(big.Float).SetUint64(0)
	var i uint64
	for i=1; i<k; i++ {	// we only do 1 to k-1 (incl.) because we only need k-1 splits
		p := rho(k, i)
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
