package ldpc

import (
	"math"
	"testing"
)

func TestNewHashRange(t *testing.T) {
	// case 1: no cyclic
	r1 := newHashRange(10, 20)
	c1 := hashRange{10, 30, false}
	if r1 != c1 {
		t.Error("incorrect non-cyclic hash range")
	}

	// case 2: cyclic
	r2 := newHashRange(math.MaxUint64-10, 20)
	c2 := hashRange{math.MaxUint64 - 10, 9, true}
	if r2 != c2 {
		t.Error("incorrect cyclic hash range")
	}
}

func TestCovers(t *testing.T) {
	// case 1: no cyclic
	r1 := newHashRange(10, 20)
	if (!r1.covers(10)) || r1.covers(40) || r1.covers(31) {
		t.Error("incorrect bound check for non-cyclic hash range")
	}

	// case 2: cyclic
	r2 := newHashRange(math.MaxUint64-10, 20)
	if (!r2.covers(math.MaxUint64)) || r2.covers(15) || r2.covers(10) {
		t.Error("incorrect bound check for cyclic hash range")
	}
}
