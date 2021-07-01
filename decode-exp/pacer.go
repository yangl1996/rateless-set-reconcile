package main

import (
	"math"
	"errors"
	"strconv"
	"strings"
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

type slientPacer struct{}

func (s *slientPacer) tick() int {
	return 0
}

func NewTransactionPacer(s string) (transactionPacer, error) {
	ds := strings.ReplaceAll(s, " ", "")
	switch {
	case strings.HasPrefix(ds, "c("):
		param := strings.TrimPrefix(strings.TrimSuffix(ds, ")"), "c(")
		rate, err := strconv.ParseFloat(param, 64)
		if err != nil {
			return nil, err
		}
		return &uniformPacer{rate, 0, 0}, nil
	case ds == "":
		return &slientPacer{}, nil
	default:
		return nil, errors.New("undefined arrival pattern")
	}

}
