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
	var sum []*big.Float
	tot := new(big.Float)
	var i uint64
	for i = 1; i <= k; i++ {
		x := new(big.Float).Copy(tot)
		sum = append(sum, x)
		tot.Add(tot, rho(k, i))
		tot.Add(tot, tau(c, delta, k, i))
	}
	var s []float64
	for i = 1; i < k; i++ {
		val, _ := new(big.Float).Quo(sum[i], tot).Float64()
		s = append(s, val)
	}
	s = append(s, 1.0)
	return &Soliton{k, s}
}

// tau implements the function tau for the robust Soliton distribution
func tau(c, delta float64, k, i uint64) *big.Float {
	r := ripple(c, delta, k)
	rf := new(big.Float).SetFloat64(r)
	th := uint64(math.Round(float64(k)/r))	// k/R
	if i < th {	// 1 to k/R-1
		ik := new(big.Float).SetUint64(i*k)
		return new(big.Float).Quo(rf, ik)
	} else if i == th {	// k/R
		log := math.Log(r) - math.Log(delta)
		logf := new(big.Float).SetFloat64(log)
		r1 := new(big.Float).Mul(rf, logf)
		return new(big.Float).Quo(r1, new(big.Float).SetUint64(k))
	} else {	// k/R+1 to k
		return new(big.Float).SetUint64(0)
	}
}

// ripple calculates the expected ripple size of a robust soliton distribution.
func ripple(c, delta float64, k uint64) float64 {
	kf := float64(k)
	res := c * math.Log(kf / delta) * math.Sqrt(kf)
	return res
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
