package ldpc

import (
	"testing"
)

// TestNewSoliton tests the creation of a soliton distribution.
func TestNewSoliton(t *testing.T) {
	s1 := NewSoliton(1)
	if len(s1.splits) != 1 || s1.splits[0] != 1.0 {
		t.Error("wrong soliton distribution for k=1")
	}

	s2 := NewSoliton(2)
	if len(s2.splits) != 2 {
		t.Error("wrong soliton distribution for k=2")
	}
	if s2.splits[0] != 0.5 || s2.splits[1] != 1.0 {
		t.Error("wrong soliton distribution for k=2")
	}

	s3 := NewSoliton(3)
	if len(s3.splits) != 3 {
		t.Error("wrong soliton distribution for k=3")
	}
	if s3.splits[0] != (1.0/3.0) || s3.splits[1] != (1.0/3.0+0.5) || s3.splits[2] != 1.0 {
		t.Error("wrong soliton distribution for k=3")
	}
}

// TestSolitonUint64 tests drawing uint64 values from soliton distribution.
func TestSolitonUint64(t *testing.T) {
	s := NewSoliton(1)
	r := s.Uint64()
	if r != 1 {
		t.Error("drawing from k=1 soliton distribution is not 1")
	}
}
