package ldpc

import (
	"golang.org/x/crypto/blake2b"
	"encoding/binary"
	"hash"
	"bytes"
	"crypto/md5"
	"sync"
)

const TxSize = 512	// the size of a transaction, including the checksum
const TxDataSize = TxSize-md5.Size	// transaction size minus the checksum size

var hasherPool = sync.Pool {
	New: func() interface{} {
		h, _ := blake2b.New512(nil)	// this fn never returns error when key=nil
		return h
	},
}

// Transaction models a transaction in the system. It embeds a checksum, used
// to simulate the signatures of real-world transactions.
type Transaction struct {
	Data [TxDataSize]byte
	checksum [md5.Size]byte
}

// NewTransaction creates a transaction from the given data by calculating
// and storing the MD5 checksum. (We use MD5 because this is a simulation
// and security does not matter.)
func NewTransaction(d [TxDataSize]byte) Transaction {
	t := Transaction{}
	copy(t.Data[:], d[:])
	t.checksum = md5.Sum(d[:])
	return t
}

// HashWithSalt calculates the hash of the transaction suffixed by the salt.
func (t *Transaction) HashWithSalt(salt []byte) []byte {
	h := hasherPool.Get().(hash.Hash)
	defer hasherPool.Put(h)
	h.Reset()
	h.Write(t.Data[:])
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
type ChecksumError struct {
	given [md5.Size]byte
	correct [md5.Size]byte
}

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
	copy(b[TxDataSize:TxSize], t.checksum[:])
	return b, nil
}

// UnmarshalBinary implements BinaryUnmarshaler. It returns an error exactly
// under two conditions: (1) the input data is unequal to TxSize (2) the
// checksum does not match the transaction data.
func (t *Transaction) UnmarshalBinary(data []byte) error {
	if len(data) != TxSize {
		return DataSizeError{len(data)}
	}
	copy(t.Data[:], data[0:TxDataSize])
	copy(t.checksum[:], data[TxDataSize:TxSize])
	cs := md5.Sum(t.Data[:])
	if bytes.Compare(cs[:], t.checksum[:]) != 0 {
		return ChecksumError{t.checksum, cs}
	} else {
		return nil
	}
}

