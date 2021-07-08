package main

import (
	"math"
	"errors"
	"strconv"
	"strings"
	"math/rand"
)

type transactionPacer interface {
	tick() int
}

type uniformPacer struct {
	rate float64
	emitted int
	generated float64
}

func (u *uniformPacer) tick() int {
	u.generated += u.rate
	t := int(math.Floor(u.generated - float64(u.emitted)))
	if t >= 1 {
		u.emitted += t
		return t
	} else {
		return 0
	}
}

type poissonPacer struct{
	rng *rand.Rand
	nextEmit float64
	curTime float64
	rate float64
}

func (s *poissonPacer) tick() int {
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
	return &poissonPacer {
		rng,
		firstEmit,
		0.0,
		rate,
	}
}

type slientPacer struct {
}

func (s *slientPacer) tick() int {
	return 0
}


func NewTransactionPacer(rng *rand.Rand, s string) (transactionPacer, error) {
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
	case ds == "":
		return &slientPacer{}, nil
	default:
		return nil, errors.New("undefined arrival pattern")
	}

}
