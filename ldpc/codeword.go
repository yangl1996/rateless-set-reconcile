package ldpc

import (
	"golang.org/x/crypto/blake2b"
	"math"
	"unsafe"
)

var emptySymbol = [TxSize]byte{}

const MaxUintIdx = blake2b.Size / 8

const (
	Into = 1  // apply a transaction into a codeword
	From = -1 // remove a transaction from a codeword
)

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	HashRange
	Counter int
	UintIdx int
	Seq     int
}

func (c *Codeword) Covers(t *HashedTransaction) bool {
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
	Dirty bool	// if we should speculate this cw again
}

func NewPendingCodeword(c Codeword) PendingCodeword {
	return PendingCodeword {
		c,
		make(map[*TimestampedTransaction]struct{}),
		make(map[*TimestampedTransaction]struct{}),
		true,
	}
}

func (c *PendingCodeword) PeelTransaction(t *TimestampedTransaction) {
	// if a transaction is already there, do not peel
	if _, there := c.Members[t]; there {
		panic("trying to peel a transaciton twice")
	}
	c.Codeword.ApplyTransaction(&t.Transaction, From)
	c.Members[t] = struct{}{}
	delete(c.Candidates, t)
	c.Dirty = true
}

func (c *PendingCodeword) UnpeelTransaction(t *TimestampedTransaction) {
	// is the transaction is not peeled, then we cannot "unpeel"
	if _, there := c.Members[t]; !there {
		panic("trying to unpeel a transaciton not peeled before")
	}
	c.Codeword.ApplyTransaction(&t.Transaction, Into)
	delete(c.Members, t)
	c.Candidates[t] = struct{}{}
	c.Dirty = true
}

func (c *PendingCodeword) AddCandidate(t *TimestampedTransaction) bool {
	_, there := c.Candidates[t]
	if there {
		return false
	} else {
		c.Candidates[t] = struct{}{}
		c.Dirty = true
		return true
	}
}

func (c *PendingCodeword) RemoveCandidate(t *TimestampedTransaction) bool {
	_, there := c.Candidates[t]
	if there {
		delete(c.Candidates, t)
		c.Dirty = true
		return true
	} else {
		return false
	}
}

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
	}
	if res < 0 {
		return math.MaxInt64
	}
	return res
}

// SpeculatePeel tries to speculatively peel off candidates from a pending
// codeword. If succeeds, it leaves the remaining transaction in the
// codeword and returns the remaining transaction with true. Otherwise, it
// returns an empty transaction and false.
func (c *PendingCodeword) SpeculatePeel() (Transaction, bool) {
	if !c.Dirty {
		return Transaction{}, false
	}
	c.Dirty = false
	var res Transaction
	// cannot peel if the remaining degree is too high (there is no enough candidate)
	if c.Counter - len(c.Candidates) > 1 {
		return res, false
	}
	// does not need peeling if the remaining degree is too low
	if c.Counter <= 1 {
		return res, false
	}
	// do not try if the cost is too high
	if c.SpeculateCost() > 100000 {
		return res, false
	}

	// collect the candidates
	candidates := make([]*TimestampedTransaction, len(c.Candidates))
	i := 0
	for k, _ := range c.Candidates {
		candidates[i] = k
		i++
	}
	// recursively try peeling off candidates
	solutions := 0
	totDepth := c.Counter - 1
	var recur func(depth int, start int) bool
	rev := false
	if totDepth < len(candidates)/2 {
		recur = func(depth int, start int) bool {
			if depth == totDepth {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(c.Symbol[:])
				if err == nil {
					res = *tx
					return true
				} else {
					solutions += 1
					return false
				}
			}
			for i := start; i < len(candidates); i++ {
				c.Codeword.ApplyTransaction(&candidates[i].Transaction, From)
				ok := recur(depth+1, i+1)
				if ok {
					// remove confirmed member from candidate list
					c.Members[candidates[i]] = struct{}{}
					delete(c.Candidates, candidates[i])
					return true
				} else {
					c.Codeword.ApplyTransaction(&candidates[i].Transaction, Into)
				}
			}
			return false
		}
	} else {
		rev = true
		for _, d := range candidates {
			c.Codeword.ApplyTransaction(&d.Transaction, From)
			c.Members[d] = struct{}{}
			delete(c.Candidates, d)
		}
		totDepth = len(candidates)-totDepth
		recur = func(depth int, start int) bool {
			if depth == totDepth {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(c.Symbol[:])
				if err == nil {
					res = *tx
					return true
				} else {
					solutions += 1
					return false
				}
			}
			for i := start; i < len(candidates); i++ {
				c.Codeword.ApplyTransaction(&candidates[i].Transaction, Into)
				ok := recur(depth+1, i+1)
				if ok {
					// remove confirmed member from candidate list
					delete(c.Members, candidates[i])
					c.Candidates[candidates[i]] = struct{}{}
					return true
				} else {
					c.Codeword.ApplyTransaction(&candidates[i].Transaction, From)
				}
			}
			return false
		}
	}
	ok := recur(0, 0)
	if ok {
		return res, true
	} else {
		if rev {
			for _, d := range candidates {
				c.Codeword.ApplyTransaction(&d.Transaction, Into)
				delete(c.Members, d)
				c.Candidates[d] = struct{}{}
			}
		}
		if solutions != c.SpeculateCost() {
			panic("unexpected number of solutions")
		}
		return res, false
	}
}

type ReleasedCodeword struct {
	Codeword
	Members []*TimestampedTransaction
}

func NewReleasedCodeword(c PendingCodeword) ReleasedCodeword {
	ls := make([]*TimestampedTransaction, len(c.Members), len(c.Members))
	idx := 0
	for k, _ := range c.Members {
		ls[idx] = k
		idx += 1
	}
	return ReleasedCodeword{c.Codeword, ls}
}

