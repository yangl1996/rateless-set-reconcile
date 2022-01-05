package ldpc

import (
	"unsafe"
)

type Codeword struct {
	symbol  [TxSize]byte	// XOR of all transactions put into this codeword
	members []uint64	// hashes of transactions; only the lowest hashLength bytes are valid
	hashLength int	// hash length in bytes
}

// XORWithTransaction XORs the transaction with the codeword symbol.
func (c *Codeword) XORWithTransaction(t *Transaction) {
	for i := 0; i < TxSize/8; i++ {
		*(*uint64)(unsafe.Pointer(&c.symbol[i*8])) ^= *(*uint64)(unsafe.Pointer(&t.serialized[i*8]))
	}
}

