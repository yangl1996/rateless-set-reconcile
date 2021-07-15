package ldpc

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"golang.org/x/crypto/blake2b"
	"hash"
	"sync"
	"unsafe"
)

const TxSize = 512                   // the size of a transaction, including the checksum
const TxDataSize = TxSize - md5.Size - 8 // transaction size minus the checksum size
const TxBodySize = TxSize - md5.Size

var hasherPool = sync.Pool{
	New: func() interface{} {
		h, _ := blake2b.New512(nil) // this fn never returns error when key=nil
		return h
	},
}

type TransactionBody struct {
	Data [TxDataSize]byte
	Timestamp uint64
}

func (t *TransactionBody) MarshalBinary() (data []byte, err error) {
	b := [TxBodySize]byte{}
	copy(b[0:TxDataSize], t.Data[:])
	binary.LittleEndian.PutUint64(b[TxDataSize:TxBodySize], t.Timestamp)
	return b[:], nil
}

func (t *TransactionBody) UnmarshalBinary(data []byte) error {
	if len(data) != TxBodySize {
		return DataSizeError{len(data)}
	}
	copy(t.Data[:], data[0:TxDataSize])
	t.Timestamp = binary.LittleEndian.Uint64(data[TxDataSize:TxBodySize])
	return nil
}

// Transaction models a transaction in the system. It embeds a checksum, used
// to simulate the signatures of real-world transactions.
type Transaction struct {
	TransactionBody
	checksum [md5.Size]byte
}

// NewTransaction creates a transaction from the given data by calculating
// and storing the MD5 checksum. (We use MD5 because this is a simulation
// and security does not matter.)
func NewTransaction(d [TxDataSize]byte, ts uint64) Transaction {
	tb := TransactionBody{d, ts}
	t := Transaction{}
	t.TransactionBody = tb
	dt, _ := tb.MarshalBinary()
	t.checksum = md5.Sum(dt)
	return t
}

// HashWithSalt calculates the hash of the transaction suffixed by the salt.
func (t *Transaction) HashWithSalt(salt []byte) []byte {
	h := hasherPool.Get().(hash.Hash)
	defer hasherPool.Put(h)
	h.Reset()
	d, _ := t.TransactionBody.MarshalBinary()
	h.Write(d)
	h.Write(t.checksum[:])
	h.Write(salt)
	return h.Sum(nil)
}

// UintWithSalt calculates the Uint64 representation of the first 8 bytes of
// the hash.
func (t *Transaction) UintWithSalt(salt []byte) uint64 {
	h := t.HashWithSalt(salt)
	return binary.LittleEndian.Uint64(h[0:8])
}

// ChecksumError catches a wrong checksum when trying to unmarshal a transaction.
type ChecksumError struct {}

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
	d, _ := t.TransactionBody.MarshalBinary()
	copy(b[0:TxBodySize], d)
	copy(b[TxBodySize:TxSize], t.checksum[:])
	return b, nil
}

// UnmarshalBinary implements BinaryUnmarshaler. It returns an error exactly
// under two conditions: (1) the input data is unequal to TxSize (2) the
// checksum does not match the transaction data.
func (t *Transaction) UnmarshalBinary(data []byte) error {
	if len(data) != TxSize {
		return DataSizeError{len(data)}
	}
	tb := TransactionBody{}
	err := (&tb).UnmarshalBinary(data[0:TxBodySize])
	if err != nil {
		return err
	}
	t.TransactionBody = tb
	copy(t.checksum[:], data[TxBodySize:TxSize])
	cs := md5.Sum(data[0:TxBodySize])
	if bytes.Compare(cs[:], t.checksum[:]) != 0 {
		return ChecksumError{}
	} else {
		return nil
	}
}

// HashedTransaction holds the transaction content and its blake2b hash.
type HashedTransaction struct {
        Transaction
        Hash [blake2b.Size]byte
}

// Uint converts the idx-th 8-byte value into an unsigned int and returns
// the result.
func (t *HashedTransaction) Uint(idx int) uint64 {
        return *(*uint64)(unsafe.Pointer(&t.Hash[idx*8]))
}

// WrapTransaction computes the hash of the given transaction, and bundles
// the hash and the transaction into a HashedTransaction.
func WrapTransaction(t Transaction) HashedTransaction {
        h := t.HashWithSalt(nil)
        tx := HashedTransaction{}
        tx.Transaction = t
        copy(tx.Hash[:], h[:])
        return tx
}
