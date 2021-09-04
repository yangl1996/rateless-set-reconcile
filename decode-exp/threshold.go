package main

import (
	"errors"
	"github.com/yangl1996/soliton"
	"math"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
)

func fracToThreshold(f float64) uint64 {
	trat := new(big.Float).SetFloat64(f)
	maxt := new(big.Float).SetUint64(math.MaxUint64)
	threshold, _ := new(big.Float).Mul(trat, maxt).Uint64()
	return threshold
}

type thresholdPicker interface {
	generate() uint64
}

type BimodalThreshold struct {
	threshold1 uint64
	threshold2 uint64
	p1         float64
	rng        *rand.Rand
}

func NewBimodalThreshold(src *rand.Rand, t1, t2, p1 float64) *BimodalThreshold {
	return &BimodalThreshold{fracToThreshold(t1), fracToThreshold(t2), p1, src}
}

func (b *BimodalThreshold) generate() uint64 {
	if b.rng.Float64() < b.p1 {
		return b.threshold1
	} else {
		return b.threshold2
	}
}

type ConstantThreshold struct {
	threshold uint64
}

func NewConstantThreshold(frac float64) *ConstantThreshold {
	return &ConstantThreshold{fracToThreshold(frac)}
}

func (c *ConstantThreshold) generate() uint64 {
	return c.threshold
}

type SolitonThreshold struct {
	dist    *soliton.Soliton
	estDiff uint64
}

func NewSolitonThreshold(rng *rand.Rand, k int, estDiff int) *SolitonThreshold {
	return &SolitonThreshold{soliton.NewSoliton(rng, uint64(k)), uint64(estDiff)}
}

func NewRobustSolitonThreshold(rng *rand.Rand, k int, c, delta float64, estDiff int) *SolitonThreshold {
	return &SolitonThreshold{soliton.NewRobustSoliton(rng, uint64(k), c, delta), uint64(estDiff)}
}

func (s *SolitonThreshold) generate() uint64 {
	return fracToThreshold(float64(s.dist.Uint64()) / float64(s.estDiff))
}

func NewDistribution(rng *rand.Rand, s string) (thresholdPicker, error) {
	ds := strings.ReplaceAll(s, " ", "")
	switch {
	case strings.HasPrefix(ds, "u("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "u(")
		frac, err := strconv.ParseFloat(param, 64)
		if err != nil {
			return nil, err
		}
		if frac > 1 || frac < 0 {
			return nil, errors.New("constant distribution fraction not in [0, 1]")
		}
		return NewConstantThreshold(frac), nil
	case strings.HasPrefix(ds, "rs("):
		params := strings.Split(strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "rs("), ",")
		if len(params) != 4 {
			return nil, errors.New("incorrect number of parameters for robust soliton")
		}
		k, err := strconv.Atoi(params[0])
		if err != nil {
			return nil, err
		}
		c, err := strconv.ParseFloat(params[1], 64)
		if err != nil {
			return nil, err
		}
		delta, err := strconv.ParseFloat(params[2], 64)
		if err != nil {
			return nil, err
		}
		dif, err := strconv.Atoi(params[3])
		if err != nil {
			return nil, err
		}
		if k <= 0 || c <= 0 || delta <= 0 || delta >= 1 || dif <= 0 {
			return nil, errors.New("parameter out of range for robust soliton")
		}
		return NewRobustSolitonThreshold(rng, k, c, delta, dif), nil
	case strings.HasPrefix(ds, "s("):
		params := strings.Split(strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "s("), ",")
		if len(params) != 2 {
			return nil, errors.New("incorrect number of parameters for soliton")
		}
		k, err := strconv.Atoi(params[0])
		if err != nil {
			return nil, err
		}
		if k <= 0 {
			return nil, errors.New("soliton distribution k not greater than 0")
		}
		dif, err := strconv.Atoi(params[1])
		if err != nil {
			return nil, err
		}
		if dif <= 0 {
			return nil, errors.New("soliton distribution diff not greater than 0")
		}
		return NewSolitonThreshold(rng, k, dif), nil
	case strings.HasPrefix(ds, "b("):
		params := strings.Split(strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "b("), ",")
		if len(params) != 3 {
			return nil, errors.New("incorrect number of parameters for bimodal")
		}
		t1, err := strconv.ParseFloat(params[0], 64)
		if err != nil {
			return nil, err
		}
		t2, err := strconv.ParseFloat(params[1], 64)
		if err != nil {
			return nil, err
		}
		p, err := strconv.ParseFloat(params[2], 64)
		if err != nil {
			return nil, err
		}
		return NewBimodalThreshold(rng, t1, t2, p), nil
	default:
		return nil, errors.New("undefined degree distribution")
	}
}
