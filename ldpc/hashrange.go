package ldpc

import (
	"math"
)

type HashRange struct {
	start, end uint64	// both bounds are inclusive
	cyclic bool
}

// NewHashRange creates a new HashRange start starts at "start" and covers
// frac+1 hash values.
func NewHashRange(start, frac uint64) HashRange {
	// frac+1 is the number of hash values accepted to this range
	cyclic := (math.MaxUint64 - start) < frac
	var end uint64
	if cyclic {
		// from start to MaxUint64: MaxUint64-start+1 hashes
		// from 0 to end: end+1 hashes
		// MaxUint64-start+1+end+1 = frac+1
		// so end = frac - (MaxUint64-start) - 1
		end = frac - (math.MaxUint64 - start) - 1
	} else {
		end = start + frac
	}
	return HashRange{start, end, cyclic}

}

// Covers checks if the hash range covers the given hash value.
func (r *HashRange) Covers(hash uint64) bool {
	if r.cyclic {
		return hash >= r.start || hash <= r.end
	} else {
		return hash >= r.start && hash <= r.end
	}
}