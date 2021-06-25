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

