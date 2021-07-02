package ldpc

import (
	"golang.org/x/crypto/blake2b"
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
	for i := 0; i < (TxSize-TxDataSize)/8; i++ {
		*(*uint64)(unsafe.Pointer(&c.Symbol[i*8+TxDataSize])) ^= *(*uint64)(unsafe.Pointer(&t.checksum[i*8]))
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
	Members map[Transaction]struct{}
}

func NewPendingCodeword(c Codeword) PendingCodeword {
	return PendingCodeword {
		c,
		make(map[Transaction]struct{}),
	}
}

func (c *PendingCodeword) PeelTransaction(t Transaction) {
	// if a transaction is already there, do not peel
	if _, there := c.Members[t]; there {
		panic("trying to peel a transaciton twice")
	}
	c.Codeword.ApplyTransaction(&t, From)
	c.Members[t] = struct{}{}
}

func (c *PendingCodeword) UnpeelTransaction(t Transaction) {
	// is the transaction is not peeled, then we cannot "unpeel"
	if _, there := c.Members[t]; !there {
		panic("trying to unpeel a transaciton not peeled before")
	}
	c.Codeword.ApplyTransaction(&t, Into)
	delete(c.Members, t)
}

func (c *PendingCodeword) SpeculatePeel(candidates []HashedTransaction) (Transaction, bool) {
	totDepth := c.Counter - 1
	var recur func(depth int, start int) bool
	var res Transaction
	recur = func(depth int, start int) bool {
		if depth == totDepth {
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				res = *tx
				return true
			} else {
				return false
			}
		}
		for i := start; i < len(candidates); i++ {
			c.PeelTransaction(candidates[i].Transaction)
			ok := recur(depth+1, i+1)
			if ok {
				return true
			} else {
				c.UnpeelTransaction(candidates[i].Transaction)
			}
		}
		return false
	}
	ok := recur(0, 0)
	if ok {
		return res, true
	} else {
		return res, false
	}
}

type ReleasedCodeword struct {
	Codeword
	Members []Transaction
}

func NewReleasedCodeword(c PendingCodeword) ReleasedCodeword {
	ls := make([]Transaction, len(c.Members), len(c.Members))
	idx := 0
	for k, _ := range c.Members {
		ls[idx] = k
		idx += 1
	}
	return ReleasedCodeword{c.Codeword, ls}
}

