package ldpc

import (
	"math"
	"unsafe"
)

var emptySymbol = [TxSize]byte{}

const (
	Into = 1  // apply a transaction into a codeword
	From = -1 // remove a transaction from a codeword
)

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	Counter int
	CodewordFilter
	Seq     int
}

type CodewordFilter struct {
	HashRange
	UintIdx int
}

// Covers returns if the hash range of the codeword filter covers the given transaction.
func (c *CodewordFilter) Covers(t *HashedTransaction) bool {
	return c.HashRange.Covers(t.Uint(c.UintIdx))
}

// ApplyTransaction adds or removes a transaction into/from the codeword,
// and increments/decrements the counter.
// d must have length TxSize, and dir must be Into or From.
func (c *Codeword) ApplyTransaction(t *Transaction, dir int) {
	for i := 0; i < TxDataSize/8; i++ {
		*(*uint64)(unsafe.Pointer(&c.Symbol[i*8])) ^= *(*uint64)(unsafe.Pointer(&t.Data[i*8]))
	}
	*(*uint64)(unsafe.Pointer(&c.Symbol[TxDataSize])) ^= t.Timestamp
	for i := 0; i < (TxSize-TxBodySize)/8; i++ {
		*(*uint64)(unsafe.Pointer(&c.Symbol[i*8+TxBodySize])) ^= *(*uint64)(unsafe.Pointer(&t.checksum[i*8]))
	}
	c.Counter += dir
}

// IsPure returns whether the codeword counter has reached zero and
// the remaining symbol is empty.
func (c *Codeword) IsPure() bool {
	if c.Counter == 0 && c.Symbol == emptySymbol {
		return true
	} else {
		return false
	}
}

type PendingCodeword struct {
	Codeword
	Candidates []*TimestampedTransaction
	Dirty bool	// if we should speculate this cw again because the candidates changed
}

func NewPendingCodeword(c Codeword) PendingCodeword {
	return PendingCodeword {
		c,
		nil,
		true,
	}
}

// PeelTransactionNotCandidate peels off a transaction t that is determined to be
// a member of c from c, assuming it is NOT already a candidate of c, and updates
// the FirstAvailable estimation for t.
func (c *PendingCodeword) PeelTransactionNotCandidate(t *TimestampedTransaction) {
	c.Codeword.ApplyTransaction(&t.Transaction, From)
	t.MarkSeenAt(c.Seq)
	c.Dirty = true
}

// AddCandidate adds a candidate transaction t to the codeword c.
func (c *PendingCodeword) AddCandidate(t *TimestampedTransaction) {
	// We would have checked if t is already in c.Candidates.
	// However, AddCandidate is only called in two situations:
	// a) when a codeword is first received; b) when a transaction
	// is first received. As a result, it's impossible that a tx
	// will be added twice.
	c.Candidates = append(c.Candidates, t)
	c.Dirty = true
	return
}

// SpeculateCost calculates the cost of speculating this codeword.
// It returns math.MaxInt64 if the cost is too high to calculate.
func (c *PendingCodeword) SpeculateCost() int {
	res := 1
	n := len(c.Candidates)
	k := c.Counter - 1
	if k > n/2 {
		k = n - k
	}
	if k >= 10 {
		return math.MaxInt64
	}
	for i := 1; i <= k; i++ {
		res = (n - k + i) * res / i
		if res < 0 {
			return math.MaxInt64
		}
	}
	return res
}

// ShouldSpeculate decides if a codeword should be speculated.
func (c *PendingCodeword) ShouldSpeculate() bool {
	// no need to run if not dirty
	if !c.Dirty {
		return false
	}
	// cannot peel if the remaining degree is too high (there is no enough candidate)
	if c.Counter - len(c.Candidates) > 1 {
		return false
	}
	// does not need peeling if the remaining degree is too low
	if c.Counter <= 1 {
		return false
	}
	// do not try if the cost is too high
	if c.SpeculateCost() > 100000 {
		return false
	}
	return true
}

// ScanCandidates scans every candidate of c, peels those whose FirstAvailable time is
// on or before c.Seq, and removes those whose LastMissing time is on or after c.Seq.
func (c *PendingCodeword) ScanCandidates() {
	cIdx := 0
	for cIdx < len(c.Candidates) {
		// Here, we should have check if txv is already a member of c,
		// and should refrain from peeling txv off c if so. However,
		// if txv is already peeled, it will NOT be a candidate in the
		// first place! (Recall that txv is the interator of c.Candidates.)
		// As a result, we do not need the check.
		txv := c.Candidates[cIdx]
		removed := false
		if c.Seq >= txv.FirstAvailable {
			// call PeelTransactionNotCandidate to peel it off and update the
			// FirstAvailable estimation. We will need to remove txv from
			// candidates manually
			c.PeelTransactionNotCandidate(txv)
			removed = true
		} else if c.Seq <= txv.LastMissing {
			removed = true
		}
		// remove txv from candidates by swapping the last candidate here
		if removed {
			c.Candidates[cIdx] = c.Candidates[len(c.Candidates)-1]
			c.Candidates = c.Candidates[0:len(c.Candidates)-1]
			c.Dirty = true
		} else {
			cIdx += 1
		}
	}
}

// tryCombinations iterates through all combinations of the candidates to leave one
// remaining transaction that is valid. It tries all subsets of size totalDepth of
// c.Candidates. If tryPeel is true, it tries peeling off the selected subset from
// the codeword; otherwise, it tries unpeeling them. The latter mode is useful when
// the number of members to discover is more than half of the codeword counter.
// It records the solutions as a strictly-increasing array of indices into c.Candidates
// in solutions, which must be pre-allocated to be of length totalDepth. It returns
// true when it successfully discovers a correct subset of members.
func (c *PendingCodeword) tryCombinations(totalDepth int, tryPeel bool, solutions []int) (*Transaction, bool) {
	tx := &Transaction{}
	nc := len(c.Candidates)
	depth := 0		// the current depth we are exploring
	firstEntry := true	// first entry into a depth after previous depths have changed
	for {
		if depth == totalDepth {
			// if we have selected a subset of size totalDepth,
			// validate the remaining tx
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				return tx, true
			} else {
				// failed; rewind the stack
				depth -= 1
				firstEntry = false
				continue
			}
		} else if depth == -1 {
			// boom; depth=0 failed or totalDepth==0
			return nil, false
		}

		if firstEntry {
			// if it's the first time we enter this depth, we should start at
			// the solution idx of the prev depth plus 1
			// however, we do not add 1 here because we will increment the
			// solution idx just below
			if depth > 0 {
				solutions[depth] = solutions[depth-1] + 1
			}
		} else {
			// if it's not the first entry into this depth after previous
			// depths have changed, we need to rollback the tx applied previously
			if tryPeel {
				c.Codeword.ApplyTransaction(&c.Candidates[solutions[depth]].Transaction, Into)
			} else {
				c.Codeword.ApplyTransaction(&c.Candidates[solutions[depth]].Transaction, From)
			}
			solutions[depth] += 1
		}
		// solutions[depth] must satisfy
		//   solutions[depth] + totalDepth - depth - 1 < nc
		// otherwise, we have run out of solutions at this depth
		// and this branch has failed
		if solutions[depth] + totalDepth - depth - 1 < nc {
			// apply this solution and advance to the next depth
			if tryPeel {
				c.Codeword.ApplyTransaction(&c.Candidates[solutions[depth]].Transaction, From)
			} else {
				c.Codeword.ApplyTransaction(&c.Candidates[solutions[depth]].Transaction, Into)
			}
			// advance into the next depth
			firstEntry = true
			depth += 1
		} else {
			firstEntry = false
			depth -= 1
		}
	}
}

// SpeculatePeel tries to speculatively peel off candidates from a pending
// codeword. If it succeeds and yields a new transaction that is not in its
// candidate set, it returns the transaction and true. The new transaction
// is not peeled. If it succeeds but
// does not yield a new transaction, i.e., all transactions are in the
// candidate set, then it return an empty transaction and false. If it fails,
// then it does not alter c and returns an empty transaction and false.
// It does nothing if the codeword c should not be speculated (see ShouldSpeculate).
func (c *PendingCodeword) SpeculatePeel() (Transaction, bool) {
	shouldRun := c.ShouldSpeculate()
	c.Dirty = false
	if !shouldRun {
		return Transaction{}, false
	}
	var res Transaction

	// depth: num of txs already peeled/unpeeled; start: start idx of candidate to look at;
	// totalDepth: total num of txs to peel/unpeel; tryPeel: true=try peeling away, false=try unpeeling;
	// solutions: indices of peeled/unpeeled transactions into candidates
	var recur func(depth, start, totalDepth int, tryPeel bool, solutions []int) bool
	recur = func(depth, start, totalDepth int, tryPeel bool, solutions []int) bool {
		if depth == totalDepth {
			// peeled enough, see if the remaining one makes sense
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				res = *tx
				return true
			} else {
				return false
			}
		}
		// iterate the ones to peel
		for i := start; i < len(c.Candidates); i++ {
			if tryPeel {
				c.Codeword.ApplyTransaction(&c.Candidates[i].Transaction, From)
			} else {
				c.Codeword.ApplyTransaction(&c.Candidates[i].Transaction, Into)
			}
			ok := recur(depth+1, i+1, totalDepth, tryPeel, solutions)
			if ok {
				// if we made the correct choice
				solutions[depth] = i
				return true
			} else {
				// if not correct, reverse the change
				if tryPeel {
					c.Codeword.ApplyTransaction(&c.Candidates[i].Transaction, Into)
				} else {
					c.Codeword.ApplyTransaction(&c.Candidates[i].Transaction, From)
				}
			}
		}
		return false

	}
	totDepth := c.Counter - 1	// number of transactions to peel; we want to leave one
	if totDepth < len(c.Candidates)/2 {
		// iterate subsets to peel
		solutions := make([]int, totDepth)
		tx, succ := c.tryCombinations(totDepth, true, solutions)
		if succ {
			res = *tx
			// register those in the solutions set
			for _, idx := range solutions {
				c.Candidates[idx].MarkSeenAt(c.Seq)
			}
			// then, try to find the remaining transaction
			// do not bother looking the ones already in solutions
			sidx := 0
			for cidx, _ := range c.Candidates {
				if sidx >= len(solutions) || cidx < solutions[sidx] {
					if res == c.Candidates[cidx].Transaction {
						// found it; peel it off
						c.PeelTransactionNotCandidate(c.Candidates[cidx])
						// clear the candidates
						c.Candidates = nil
						c.Dirty = false
						return res, false
					}
				} else if cidx == solutions[sidx] {
					sidx += 1
				} else {
					panic("how")
				}
			}
			// failed to locate the remaining tx (res) in candidates; it must be new
			// clear the candidates
			c.Candidates = nil
			c.Dirty = false
			return res, true
		} else {
			return res, false
		}
	} else {
		// iterate subsets to NOT peel
		// first apply every candidate
		for _, d := range c.Candidates {
			c.Codeword.ApplyTransaction(&d.Transaction, From)
		}
		totDepth = len(c.Candidates)-totDepth
		solutions := make([]int, totDepth)
		tx, succ := c.tryCombinations(totDepth, false, solutions)
		if succ {
			res = *tx
			// solutions contains the set of indices we DO NOT want to peel
			// here, we register those that we DO want to peel, i.e., those
			// in candidates but not in solutions
			sidx := 0
			for cidx, _ := range c.Candidates {
				if sidx >= len(solutions) || cidx < solutions[sidx] {
					c.Candidates[cidx].MarkSeenAt(c.Seq)
				} else if cidx == solutions[sidx] {
					sidx += 1
				} else {
					panic("how!")
				}
			}
			// then, try to find the remaining one (res) among the ones
			// not peeled
			for _, idx := range solutions {
				if res == c.Candidates[idx].Transaction {
					// found it; peel it off
					c.PeelTransactionNotCandidate(c.Candidates[idx])
					c.Dirty = false
					c.Candidates = nil
					return res, false
				}
			}
			// not found
			c.Dirty = false
			c.Candidates = nil
			return res, true
		} else {
			for _, d := range c.Candidates {
				c.Codeword.ApplyTransaction(&d.Transaction, Into)
			}
			return res, false
		}
	}
}

type ReleasedCodeword struct {
	CodewordFilter
	Seq     int
}

func NewReleasedCodeword(c *PendingCodeword) ReleasedCodeword {
	return ReleasedCodeword{c.CodewordFilter, c.Seq}
}

