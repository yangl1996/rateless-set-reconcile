package main

import (
	"testing"
)

func TestParseConstantDistribution(t *testing.T) {
	s1 := "u( 0.01  )"
	d1, err := NewDistribution(s1, 100, 0)
	if err != nil {
		t.Error(err)
	}
	u := d1.(*ConstantThreshold)
	if u.threshold != fracToThreshold(0.01) {
		t.Error("wrong constant threshold")
	}
}

func compareSoliton(d1, d2 *SolitonThreshold) bool {
	if d1.estDiff != d2.estDiff {
		return false
	}
	if d1.dist.Equals(d2.dist) {
		return true
	}
	return false
}

func TestParseSolitonDistribution(t *testing.T) {
	s1 := "s( 10  )"
	d, err := NewDistribution(s1, 100, 0)
	if err != nil {
		t.Error(err)
	}
	d1 := d.(*SolitonThreshold)
	d2 := NewSolitonThreshold(0, 10, 100)
	if !compareSoliton(d1, d2) {
		t.Error("wrong soliton threshold")
	}
}

func TestParseRobustSolitonDistribution(t *testing.T) {
	s1 := "rs( 10, 0.1, 0.001  )"
	d, err := NewDistribution(s1, 100, 0)
	if err != nil {
		t.Error(err)
	}
	d1 := d.(*SolitonThreshold)
	d2 := NewRobustSolitonThreshold(0, 10, 0.1, 0.001, 100)
	if !compareSoliton(d1, d2) {
		t.Error("wrong robust soliton threshold")
	}
}
