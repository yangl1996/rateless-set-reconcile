package main

import (
	"errors"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

type pacer interface {
	tick(int) int
}

type uniformPacer struct {
	rate      float64
	emitted   int
	generated float64
}

func (u *uniformPacer) tick(_ int) int {
	u.generated += u.rate
	t := int(math.Floor(u.generated - float64(u.emitted)))
	if t >= 1 {
		u.emitted += t
		return t
	} else {
		return 0
	}
}

type poissonPacer struct {
	rng      *rand.Rand
	nextEmit float64
	curTime  float64
	rate     float64
}

func (s *poissonPacer) tick(_ int) int {
	s.curTime += 1.0
	t := 0
	for s.nextEmit <= s.curTime {
		t += 1
		s.nextEmit += s.rng.ExpFloat64() / s.rate
	}
	return t
}

func NewPoissonPacer(rng *rand.Rand, rate float64) *poissonPacer {
	if rng == nil {
		// called because we are just validating the syntax
		return nil
	}
	firstEmit := rng.ExpFloat64() / rate
	return &poissonPacer{
		rng,
		firstEmit,
		0.0,
		rate,
	}
}

type slientPacer struct {
}

func (s *slientPacer) tick(_ int) int {
	return 0
}

type countingPacer struct {
	cnt int
}

func (s *countingPacer) tick(n int) int {
	if n >= s.cnt {
		return 0
	} else {
		return s.cnt - n
	}
}

func NewTransactionPacer(rng *rand.Rand, s string) (pacer, error) {
	ds := strings.ReplaceAll(s, " ", "")
	switch {
	case strings.HasPrefix(ds, "c("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "c(")
		rate, err := strconv.ParseFloat(param, 64)
		if err != nil {
			return nil, err
		}
		return &uniformPacer{rate, 0, 0}, nil
	case strings.HasPrefix(ds, "p("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "p(")
		rate, err := strconv.ParseFloat(param, 64)
		if err != nil {
			return nil, err
		}
		return NewPoissonPacer(rng, rate), nil
	case strings.HasPrefix(ds, "n("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "n(")
		p, err := strconv.Atoi(param)
		if err != nil {
			return nil, err
		}
		return &countingPacer{p}, nil
	case ds == "":
		return &slientPacer{}, nil
	default:
		return nil, errors.New("undefined arrival pattern")
	}

}
