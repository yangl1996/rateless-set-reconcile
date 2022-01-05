package ldpc

import (
	"encoding/binary"
	"hash"
	"sync"

	"golang.org/x/crypto/blake2b"
)

const TxSize = 512	// size of a serialized transaction in bytes

var hasherPool = sync.Pool{
	New: func() interface{} {
		h, _ := blake2b.New512(nil) // this fn never returns error when key=nil
		return h
	},
}

type Transaction struct {
	serialized [TxSize]byte
	hash [blake2b.Size]byte
	shortHash uint64
}

func (t *Transaction) UnmarshalBinary(data []byte) error {
	// check transaction size
	if len(data) != TxSize {
		return DataSizeError{len(data)}
	}
	copied := copy(t.serialized[0:TxSize], data[0:TxSize])
	if copied != TxSize {
		panic("incorrect number of bytes copied")
	}

	h := hasherPool.Get().(hash.Hash)
	defer hasherPool.Put(h)
	h.Reset()
	h.Write(t.serialized[:])
	h.Sum(t.hash[0:0])	// Sum appends to the given slice
	t.shortHash = binary.BigEndian.Uint64(t.hash[0:8])
	return nil
}

// uint converts the first l bytes of the transaction hash into an unsigned int and returns
// the result.
func (t *Transaction) Uint64(l int) uint64 {
	if l > 8 {
		panic("hash size exceeds 8 bytes")
	}
	return t.shortHash & ((0x01 << (l << 3)) - 1)
}

type DataSizeError struct {
	length int
}

func (e DataSizeError) Error() string {
	return "incorrect data size given to unmarshaler"
}
