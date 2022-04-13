package ldpc

import (
	"hash"
	"sync"
	"unsafe"

	"golang.org/x/crypto/blake2b"
)

const TxSize = 128 // size of a serialized transaction in bytes

type TransactionData [TxSize]byte

func (t *TransactionData) XOR(t2 *TransactionData) {
	for i := 0; i < TxSize/8; i++ {
		*(*uint64)(unsafe.Pointer(&t[i*8])) ^= *(*uint64)(unsafe.Pointer(&t2[i*8]))
	}
}

var hasherPool = sync.Pool{
	New: func() interface{} {
		h, _ := blake2b.New512(nil) // this fn never returns error when key=nil
		return h
	},
}

type Transaction struct {
	serialized TransactionData
	hash       [blake2b.Size]byte
}

func (t *Transaction) Serialized() []byte {
	return t.serialized[:]
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
	h.Sum(t.hash[0:0]) // Sum appends to the given slice
	return nil
}

type DataSizeError struct {
	length int
}

func (e DataSizeError) Error() string {
	return "incorrect data size given to unmarshaler"
}
