package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"math"
	"math/big"
	"strings"
	"strconv"
	"errors"
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
	dist *ldpc.Soliton
	estDiff uint64
}

func NewSolitonThreshold(k int, estDiff int) *SolitonThreshold {
	return &SolitonThreshold{ldpc.NewSoliton(uint64(k)), uint64(estDiff)}
}

func NewRobustSolitonThreshold(k int, c, delta float64, estDiff int) *SolitonThreshold {
	return &SolitonThreshold{ldpc.NewRobustSoliton(uint64(k), c, delta), uint64(estDiff)}
}

func (s *SolitonThreshold) generate() uint64 {
	return fracToThreshold(float64(s.dist.Uint64()) / float64(s.estDiff))
}

func NewDistribution(s string, estDiff int) (thresholdPicker, error) {
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
		if len(params) != 3 {
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
		if k <= 0 || c <= 0 || delta <= 0 || delta >= 1 {
			return nil, errors.New("parameter out of range for robust soliton")
		}
		return NewRobustSolitonThreshold(k, c, delta, estDiff), nil
	case strings.HasPrefix(ds, "s("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "s(")
		k, err := strconv.Atoi(param)
		if err != nil {
			return nil, err
		}
		if k <= 0 {
			return nil, errors.New("soliton distribution k not greater than 0")
		}
		return NewSolitonThreshold(k, estDiff), nil
	default:
		return nil, errors.New("undefined degree distribution")
	}
}
