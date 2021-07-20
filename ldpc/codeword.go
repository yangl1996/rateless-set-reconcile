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
	Members map[*TimestampedTransaction]struct{}
	Candidates map[*TimestampedTransaction]struct{}
	Dirty bool	// if we should speculate this cw again because the candidates changed
}

func NewPendingCodeword(c Codeword) PendingCodeword {
	return PendingCodeword {
		c,
		make(map[*TimestampedTransaction]struct{}, c.Counter),
		make(map[*TimestampedTransaction]struct{}, c.Counter),
		true,
	}
}

// PeelTransaction peels off a transaction t that is determined to be
// a member of c from c, updates the members and candidates of c, and
// updates the FirstAvailable estimation of t. It must not be called
// if c is already peeled from t.
func (c *PendingCodeword) PeelTransaction(t *TimestampedTransaction) {
	c.Codeword.ApplyTransaction(&t.Transaction, From)
	if _, there := c.Candidates[t]; there {
		delete(c.Candidates, t)
		c.Dirty = true
	}
	c.Members[t] = struct{}{}
	if t.FirstAvailable > c.Seq {
		t.FirstAvailable = c.Seq
	}
}

// AddCandidate adds a candidate transaction t to the codeword c.
func (c *PendingCodeword) AddCandidate(t *TimestampedTransaction) {
	_, there := c.Candidates[t]
	if there {
		return
	} else {
		c.Candidates[t] = struct{}{}
		c.Dirty = true
		return
	}
}

// RemoveCandidate removes a candidate transaction t from the codeword c.
func (c *PendingCodeword) RemoveCandidate(t *TimestampedTransaction) {
	_, there := c.Candidates[t]
	if there {
		delete(c.Candidates, t)
		c.Dirty = true
		return
	} else {
		return
	}
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

	// collect the candidates
	candidates := make([]*TimestampedTransaction, len(c.Candidates))
	i := 0
	for k, _ := range c.Candidates {
		candidates[i] = k
		i++
	}
	// depth: num of txs already peeled/unpeeled; start: start idx of candidate to look at;
	// totalDepth: total num of txs to peel/unpeel; tryPeel: true=try peeling away, false=try unpeeling.
	var recur func(depth, start, totalDepth int, tryPeel bool) bool
	recur = func(depth, start, totalDepth int, tryPeel bool) bool {
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
		for i := start; i < len(candidates); i++ {
			if tryPeel {
				c.Codeword.ApplyTransaction(&candidates[i].Transaction, From)
			} else {
				c.Codeword.ApplyTransaction(&candidates[i].Transaction, Into)
			}
			ok := recur(depth+1, i+1, totalDepth, tryPeel)
			if ok {
				// if we made the correct choice
				if tryPeel {
					c.Members[candidates[i]] = struct{}{}
					delete(c.Candidates, candidates[i])
					return true
				} else {
					delete(c.Members, candidates[i])
					c.Candidates[candidates[i]] = struct{}{}
					return true
				}
			} else {
				// if not correct, reverse the change
				if tryPeel {
					c.Codeword.ApplyTransaction(&candidates[i].Transaction, Into)
				} else {
					c.Codeword.ApplyTransaction(&candidates[i].Transaction, From)
				}
			}
		}
		return false

	}
	totDepth := c.Counter - 1	// number of transactions to peel; we want to leave one
	if totDepth < len(candidates)/2 {
		if recur(0, 0, totDepth, true) {
			return res, true
		} else {
			return res, false
		}
	} else {
		// iterate subsets to NOT peel
		// first apply every candidate
		for _, d := range candidates {
			c.Codeword.ApplyTransaction(&d.Transaction, From)
			c.Members[d] = struct{}{}
			delete(c.Candidates, d)
		}
		totDepth = len(candidates)-totDepth
		if recur(0, 0, totDepth, false) {
			return res, true
		} else {
			for _, d := range candidates {
				c.Codeword.ApplyTransaction(&d.Transaction, Into)
				delete(c.Members, d)
				c.Candidates[d] = struct{}{}
			}
			return res, false
		}
	}
}

type ReleasedCodeword struct {
	CodewordFilter
	Seq     int
}

func NewReleasedCodeword(c PendingCodeword) ReleasedCodeword {
	return ReleasedCodeword{c.CodewordFilter, c.Seq}
}

