package ldpc

import (
	"testing"
	"math"
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

// TestNewRobustSoliton tests the creation of a robust soliton distribution.
func TestNewRobustSoliton(t *testing.T) {
	s1 := NewRobustSoliton(1, 1.2, 0.001)
	if len(s1.splits) != 1 || s1.splits[0] != 1.0 {
		t.Error("wrong soliton distribution for k=1")
	}

	s2 := NewRobustSoliton(3, 0.12, 0.001)
	// R = 1.6640922493490253214333366552672665558709784486254181035332
	// Threshold = 1.80 = 2
	// Tau = 0.5546974164496751071444455517557555186236594828751393678444
	//       4.1142101844753662286537231656606822751670098629800196368954
	//       0
	// Rho = 1/3
	//       1/2
	//       1/6
	// Tau + Rho = 0.8880307497830084404777788850890888519569928162084727011777333333
	//             4.6142101844753662286537231656606822751670098629800196368954
	//             1/6
	// Sum = 5.6689076009250413357981687174164377937906693458551590047397999999
	if len(s2.splits) != 3 {
		t.Error("wrong soliton distribution for k=3")
	}
	t.Log(s2.splits[0])
	t.Log(s2.splits[1])
	t.Log(s2.splits[2])
	e1 := math.Abs(s2.splits[0] - 0.1566493603878993030143281532615261342211803355562430962538559755) < 0.000001
	e2 := math.Abs(s2.splits[1] - 0.9705998618429641852703276471816809044716254319807433471959590362) < 0.000001
	e3 := math.Abs(s2.splits[2] - 1.0) < 0.000001

	if !(e1 && e2 && e3) {
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

	// test the sanity check that k should be larger than k
	// we want to test if the function panicked. we want it to panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("uint64 did not panic when returning a value larger than k")
		}
	}()
	s.k = 0
	s.Uint64()
}

// TestSolitonEqual tests the comparator of two Soliton distributions.
func TestSolitonEqual(t *testing.T) {
	s1 := NewSoliton(4)
	s2 := NewSoliton(4)
	if s1.Equals(s2) != true {
		t.Error("comparator returns false when two distributions equal")
	}

	s3 := NewSoliton(5)
	if s1.Equals(s3) != false {
		t.Error("comparator returns true when two distributions differ")
	}
	s4 := NewSoliton(5)
	s4.k = 4	// we want to trigger the slice length check
	if s1.Equals(s4) != false {
		t.Error("comparator returns true when two distributions differ")
	}
	s5 := NewRobustSoliton(4, 0.01, 0.02)
	if s1.Equals(s5) != false {
		t.Error("comparator returns true when two distributions differ")
	}

}
