package ldpc

import (
	"encoding/binary"
	"hash"
	"sync"
	"unsafe"

	"github.com/cespare/xxhash"
	"golang.org/x/crypto/blake2b"
)

const checksumSize = 8
const TxSize = 512                           // the size of a transaction, including the checksum
const TxDataSize = TxSize - checksumSize - 8 // transaction size minus the checksum size
const txBodySize = TxSize - checksumSize

var hasherPool = sync.Pool{
	New: func() interface{} {
		h, _ := blake2b.New512(nil) // this fn never returns error when key=nil
		return h
	},
}

var checksumPool = sync.Pool{
	New: func() interface{} {
		h := xxhash.New()
		return h
	},
}

type TransactionBody struct {
	Data      [TxDataSize]byte
	Timestamp uint64
}

func (t *TransactionBody) MarshalBinary() (data []byte, err error) {
	b := [txBodySize]byte{}
	copy(b[0:TxDataSize], t.Data[:])
	binary.LittleEndian.PutUint64(b[TxDataSize:txBodySize], t.Timestamp)
	return b[:], nil
}

func (t *TransactionBody) UnmarshalBinary(data []byte) error {
	if len(data) != txBodySize {
		return DataSizeError{len(data)}
	}
	copy(t.Data[:], data[0:TxDataSize])
	t.Timestamp = binary.LittleEndian.Uint64(data[TxDataSize:txBodySize])
	return nil
}

// Transaction models a transaction in the system. It embeds a checksum, used
// to simulate the signatures of real-world transactions.
type Transaction struct {
	TransactionBody
	checksum uint64
}

// NewTransaction creates a transaction from the given data by calculating
// and storing the MD5 checksum. (We use MD5 because this is a simulation
// and security does not matter.)
func NewTransaction(d [TxDataSize]byte, ts uint64) Transaction {
	t := Transaction{
		TransactionBody: TransactionBody{
			Data:      d,
			Timestamp: ts,
		},
	}
	h := checksumPool.Get().(hash.Hash64)
	defer checksumPool.Put(h)
	h.Reset()
	h.Write(((*[TxDataSize]byte)(noescape(unsafe.Pointer(&t.Data[0]))))[:])
	h.Write(((*[8]byte)(noescape(unsafe.Pointer(&t.Timestamp))))[:])
	t.checksum = h.Sum64()
	return t
}

// noescape hides a pointer from escape analysis. I learned this trick
// from https://segment.com/blog/allocation-efficiency-in-high-performance-go-services/
// and the origin is https://golang.org/src/strings/builder.go
// this is crazy...
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

// hashWithSaltInto calculates the hash of the transaction suffixed by the salt
// and writes into dst.
func (t *Transaction) hashWithSaltInto(salt []byte, dst *[blake2b.Size]byte) {
	h := hasherPool.Get().(hash.Hash)
	defer hasherPool.Put(h)
	h.Reset()
	h.Write(t.Data[:])
	h.Write((*[8]byte)(unsafe.Pointer(&t.Timestamp))[:])
	h.Write((*[8]byte)(unsafe.Pointer(&t.checksum))[:])
	h.Write(salt)
	h.Sum(dst[0:0])
	return
}

// hashWithSalt calculates the hash of the transaction suffixed by the salt.
func (t *Transaction) hashWithSalt(salt []byte) [blake2b.Size]byte {
	var res [blake2b.Size]byte
	t.hashWithSaltInto(salt, &res)
	return res
}

// uintWithSalt calculates the Uint64 representation of the first 8 bytes of
// the hash.
func (t *Transaction) uintWithSalt(salt []byte) uint64 {
	h := t.hashWithSalt(salt)
	return binary.LittleEndian.Uint64(h[0:8])
}

// ChecksumError catches a wrong checksum when trying to unmarshal a transaction.
type ChecksumError struct{}

func (e ChecksumError) Error() string {
	return "incorrect transaction checksum"
}

// DataSizeError is returned when trying to unmarshal a byte slice that
// is not TxSize in length.
type DataSizeError struct {
	length int
}

func (e DataSizeError) Error() string {
	return "incorrect data size given to unmarshaler"
}

// MarshalBinary implements BinaryMarshaler. It always return
// a byte array of TxSize and the error is always nil.
func (t *Transaction) MarshalBinary() (data []byte, err error) {
	b := make([]byte, TxSize)
	copy(b[0:TxDataSize], t.Data[:])
	copy(b[TxDataSize:txBodySize], (*[8]byte)(unsafe.Pointer(&t.Timestamp))[:])
	copy(b[txBodySize:TxSize], (*[8]byte)(unsafe.Pointer(&t.checksum))[:])
	return b, nil
}

// UnmarshalBinary implements BinaryUnmarshaler. It returns an error exactly
// under two conditions: (1) the input data is unequal to TxSize (2) the
// checksum does not match the transaction data.
func (t *Transaction) UnmarshalBinary(data []byte) error {
	// check transaction size
	if len(data) != TxSize {
		return DataSizeError{len(data)}
	}
	// check the checksum; we write the computed checksum into t
	// to avoid allocating [ChecksumSize]byte
	h := checksumPool.Get().(hash.Hash64)
	defer checksumPool.Put(h)
	h.Reset()
	h.Write(data[0:txBodySize])
	cs := h.Sum64()
	if *(*uint64)(unsafe.Pointer(&data[txBodySize])) != cs {
		return ChecksumError{}
	} else {
		t.checksum = cs
		return (&t.TransactionBody).UnmarshalBinary(data[0:txBodySize])
	}
}

// hashedTransaction holds the transaction content and its blake2b hash.
type hashedTransaction struct {
	Transaction
	hash [blake2b.Size]byte
}

// uint converts the idx-th 8-byte value into an unsigned int and returns
// the result.
func (t *hashedTransaction) uint(idx int) uint64 {
	return *(*uint64)(unsafe.Pointer(&t.hash[idx*8]))
}
