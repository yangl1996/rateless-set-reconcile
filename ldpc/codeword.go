package ldpc

import (
	"math"
	"unsafe"
)

var emptySymbol = [TxSize]byte{}

const (
	into = 1  // apply a transaction into a codeword
	from = -1 // remove a trasansaction from a codeword
)

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	symbol  [TxSize]byte
	counter int
	hashRangeFilter
	timestamp uint64
	members bloom
}

type hashRangeFilter struct {
	hashRange
	hashIdx      int
}

func (c *hashRangeFilter) covers(t *hashedTransaction) bool {
	return c.hashRange.covers(t.uint(c.hashIdx))
}

// applyTransaction adds or removes a transaction into/from the codeword,
// and increments/decrements the counter.
// d must have length TxSize, and dir must be Into or From.
func (c *Codeword) applyTransaction(t *Transaction, dir int) {
	for i := 0; i < TxDataSize/8; i++ {
		*(*uint64)(unsafe.Pointer(&c.symbol[i*8])) ^= *(*uint64)(unsafe.Pointer(&t.Data[i*8]))
	}
	*(*uint64)(unsafe.Pointer(&c.symbol[txBodySize])) ^= t.checksum
	c.counter += dir
}

// isPure returns whether the codeword counter has reached zero and
// the remaining symbol is empty.
func (c *Codeword) isPure() bool {
	if c.counter == 0 && c.symbol == emptySymbol {
		return true
	} else {
		return false
	}
}

type pendingCodeword struct {
	Codeword
	candidates  []*timestampedTransaction
	dirty       bool // if we should speculate this cw again because the candidates changed
}

func (c *pendingCodeword) removeCandidateAt(idx int) {
	newLen := len(c.candidates) - 1
	c.candidates[idx].rc -= 1
	if c.candidates[idx].rc == 0 {
		c.candidates[idx].hashedTransaction.rc -= 1
		if c.candidates[idx].hashedTransaction.rc == 0 {
			hashedTransactionPool.Put(c.candidates[idx].hashedTransaction)
			c.candidates[idx].hashedTransaction = nil
		}
		timestampPool.Put(c.candidates[idx])
	}
	c.candidates[idx] = c.candidates[newLen]
	c.candidates[newLen] = nil
	c.candidates = c.candidates[0:newLen]
}

// peelTransactionNotCandidate peels off a transaction t that is determined to be
// a member of c from c, assuming it is NOT already a candidate of c, and updates
// the FirstAvailable estimation for t.
func (c *pendingCodeword) peelTransactionNotCandidate(t *timestampedTransaction) {
	c.Codeword.applyTransaction(&t.Transaction, from)
	t.markSeenAt(c.timestamp)
	c.dirty = true
}

// addCandidate adds a candidate transaction t to the codeword c.
func (c *pendingCodeword) addCandidate(t *timestampedTransaction) {
	// We would have checked if t is already in c.Candidates.
	// However, AddCandidate is only called in two situations:
	// a) when a codeword is first received; b) when a transaction
	// is first received. As a result, it's impossible that a tx
	// will be added twice.
	c.candidates = append(c.candidates, t)
	c.dirty = true
	t.rc += 1
	return
}

// noSpecDepth is the max number of missing transactions that we try speculate
// in a codeword.
const noSpecDepth = 10

// speculateCost calculates the cost of speculating this codeword.
// It returns math.MaxInt64 if the cost is too high to calculate.
func (c *pendingCodeword) speculateCost() int {
	res := 1
	n := len(c.candidates)
	k := c.counter - 1
	if k > n/2 {
		k = n - k
	}
	if k >= noSpecDepth {
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

// shouldSpeculate decides if a codeword should be speculated.
func (c *pendingCodeword) shouldSpeculate() bool {
	// no need to run if not dirty
	if !c.dirty {
		return false
	}
	// cannot peel if the remaining degree is too high (there is no enough candidate)
	if c.counter-len(c.candidates) > 1 {
		return false
	}
	// does not need peeling if the remaining degree is too low
	if c.counter <= 1 {
		return false
	}
	// do not try if the cost is too high
	if c.speculateCost() > 100000 {
		return false
	}
	return true
}

// scanCandidates scans every candidate of c, peels those whose FirstAvailable time is
// on or before c.Seq, and removes those whose LastMissing time is on or after c.Seq.
func (c *pendingCodeword) scanCandidates() {
	cIdx := 0
	for cIdx < len(c.candidates) {
		// Here, we should have check if txv is already a member of c,
		// and should refrain from peeling txv off c if so. However,
		// if txv is already peeled, it will NOT be a candidate in the
		// first place! (Recall that txv is the interator of c.Candidates.)
		// As a result, we do not need the check.
		txv := c.candidates[cIdx]
		removed := false
		if c.timestamp >= txv.firstAvailable && c.timestamp <= txv.lastInclude {
			// call PeelTransactionNotCandidate to peel it off and update the
			// FirstAvailable estimation. We will need to remove txv from
			// candidates manually
			c.peelTransactionNotCandidate(txv)
			removed = true
		} else if c.timestamp <= txv.lastMissing || c.timestamp >= txv.firstDrop {
			removed = true
		}
		// remove txv from candidates by swapping the last candidate here
		if removed {
			c.removeCandidateAt(cIdx)
			c.dirty = true
		} else {
			cIdx += 1
		}
	}
}

// tryCombinations iterates through all combinations of the candidates to leave one
// remaining transaction that is valid. It tries all subsets of size totalDepth of
// c.Candidates. If tryPeel is true, it tries peeling off the selected subset from
// the codeword; otherwise, it tries unpeeling them. The second mode is useful when
// the number of members to discover is more than half of the codeword counter, where
// the caller should first peel all candidates from c (leaving c.Counter <= 0).
// It records the solutions as a strictly-increasing array of indices into c.Candidates
// in solutions, which must be pre-allocated to be of length totalDepth. It returns
// true when it successfully discovers a correct subset of members.
func (c *pendingCodeword) tryCombinations(totalDepth int, tryPeel bool, solutions []int) (Transaction, bool) {
	tx := Transaction{}
	nc := len(c.candidates)
	depth := 0         // the current depth we are exploring
	firstEntry := true // first entry into a depth after previous depths have changed
	for {
		if depth == totalDepth {
			// if we have selected a subset of size totalDepth,
			// validate the remaining tx
			err := (&tx).UnmarshalBinary(c.symbol[:])
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
			return tx, false
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
				c.Codeword.applyTransaction(&c.candidates[solutions[depth]].Transaction, into)
			} else {
				c.Codeword.applyTransaction(&c.candidates[solutions[depth]].Transaction, from)
			}
			solutions[depth] += 1
		}
		// solutions[depth] must satisfy
		//   solutions[depth] + totalDepth - depth - 1 < nc
		// otherwise, we have run out of solutions at this depth
		// and this branch has failed
		if solutions[depth]+totalDepth-depth-1 < nc {
			// apply this solution and advance to the next depth
			if tryPeel {
				c.Codeword.applyTransaction(&c.candidates[solutions[depth]].Transaction, from)
			} else {
				c.Codeword.applyTransaction(&c.candidates[solutions[depth]].Transaction, into)
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

// speculatePeel tries to speculatively peel off candidates from a pending
// codeword. If it succeeds and yields a new transaction that is not in its
// candidate set, it returns the transaction and true. The new transaction
// is not peeled. If it succeeds but
// does not yield a new transaction, i.e., all transactions are in the
// candidate set, then it return an empty transaction and false. If it fails,
// then it does not alter c and returns an empty transaction and false.
func (c *pendingCodeword) speculatePeel() (Transaction, bool) {
	var res Transaction
	var succ bool

	totDepth := c.counter - 1 // number of transactions to peel; we want to leave one
	if totDepth < len(c.candidates)/2 {
		// iterate subsets to peel
		var solutions []int
		var solBack [noSpecDepth]int // pre-allocate an array on the stack to back solutions
		solutions = solBack[0:totDepth]
		res, succ = c.tryCombinations(totDepth, true, solutions)
		if succ {
			// Register those in the solutions set and remove them from the
			// candidates list. We would like to do both in one pass.
			// To do that, we go backward wrt the index into Candidates.
			// We cannot go forward because we will be pulling candidates from
			// the end of the set when deleting items in the front; such pulled
			// items may need to be deleted again. Going backwards does not have
			// this problem.
			for sidx := len(solutions) - 1; sidx >= 0; sidx-- {
				c.candidates[solutions[sidx]].markSeenAt(c.timestamp)
				c.removeCandidateAt(solutions[sidx])
			}
		} else {
			return res, false
		}
	} else {
		// iterate subsets to NOT peel
		// first apply every candidate
		for _, d := range c.candidates {
			c.Codeword.applyTransaction(&d.Transaction, from)
		}
		totDepth = len(c.candidates) - totDepth
		var solutions []int
		var solBack [noSpecDepth]int // pre-allocate an array on the stack to back solutions
		solutions = solBack[0:totDepth]
		res, succ = c.tryCombinations(totDepth, false, solutions)
		if succ {
			// solutions[] contains the set of indices we DO NOT want to peel
			// here; we want to leave them alone, and MarkSeen/delete those
			// we DO need to peel, i.e., those not in solutions[].
			// As a solution, we go backwards wrt index into Candidates, and
			// skip those which also exist in solutions.
			sidx := len(solutions) - 1
			for cidx := len(c.candidates) - 1; cidx >= 0; cidx-- {
				if sidx < 0 || cidx > solutions[sidx] {
					c.candidates[cidx].markSeenAt(c.timestamp)
					c.removeCandidateAt(cidx)
				} else if cidx == solutions[sidx] {
					sidx -= 1
				}
			}
		} else {
			for _, d := range c.candidates {
				c.Codeword.applyTransaction(&d.Transaction, into)
			}
			return res, false
		}
	}
	c.dirty = false
	// then, try to find the remaining transaction; we have removed
	// the solutions from Candidates, so Candidates only contains non-members.
	for cidx := range c.candidates {
		if res.checksum == c.candidates[cidx].Transaction.checksum && res == c.candidates[cidx].Transaction {
			// found it; peel it off (which marks FirstAvailable for us)
			c.peelTransactionNotCandidate(c.candidates[cidx])
			c.removeCandidateAt(cidx)
			// no more member to look for; we can return
			return res, false
		}
	}
	// failed to locate the remaining tx (res) in candidates; it must be new
	return res, true
}

