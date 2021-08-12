package ldpc

import (
	"math"

	"golang.org/x/crypto/blake2b"
)

const MaxHashIdx = blake2b.Size / 8

type hashRange struct {
	start, end uint64 // both bounds are inclusive
	cyclic     bool
}

// newHashRange creates a new HashRange start starts at "start" and covers
// frac+1 hash values.
func newHashRange(start, frac uint64) hashRange {
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
	return hashRange{start, end, cyclic}

}

// covers checks if the hash range covers the given hash value.
func (r *hashRange) covers(hash uint64) bool {
	if r.cyclic {
		return hash >= r.start || hash <= r.end
	} else {
		return hash >= r.start && hash <= r.end
	}
}

// bucketIndexRange returns the starting and ending indices of buckets.
// Both indices are inclusive and the caller must take the remainder
// against NumBuckets before using.
func (r *hashRange) bucketIndexRange() (int, int) {
	start := int(r.start / bucketSize)
	end := int(r.end / bucketSize)
	if r.cyclic {
		if end == start {
			return 0, numBuckets - 1
		} else if end < start {
			return start, end + numBuckets
		} else {
			panic("corrupted")
		}
	} else {
		return start, end
	}
}
