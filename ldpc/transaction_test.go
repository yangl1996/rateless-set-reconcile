package ldpc

import (
	"math/rand"
	"testing"
	"bytes"
	"crypto/md5"
	"golang.org/x/crypto/blake2b"
	"encoding/binary"
)

func randomData() [TxDataSize]byte {
	d := [TxDataSize]byte{}
	rand.Read(d[:])
	return d
}

// TestNewTransaction tests the creation of a transaction.
func TestNewTransaction(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d)
	h := md5.Sum(tx.Data[:])
	if bytes.Compare(h[:], tx.checksum[:]) != 0 {
		t.Error("incorrect checksum in created transaction")
	}
}

// TestHashingAndUint tests the hashing and uint with salt.
func TestHashingAndUint(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d)
	s := []byte{}
	s = append(s, tx.Data[:]...)
	s = append(s, tx.checksum[:]...)
	s = append(s, 1, 2, 3)	// salt
	hash := blake2b.Sum256(s)
	salt := []byte{1, 2, 3}
	given := tx.HashWithSalt(salt)
	if bytes.Compare(hash[:], given[:]) != 0 {
		t.Error("incorrect hash result")
	}
	itg := binary.LittleEndian.Uint64(hash[0:8])
	gitg := tx.UintWithSalt(salt)
	if itg != gitg {
		t.Error("incorrect uint64 result")
	}
}

// TestMarshal tests the marshalling and unmarshalling of a transaction.
func TestMarshal(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d)
	m, err := tx.MarshalBinary()
	if err != nil {
		t.Error("error marshalling transaction")
	}
	un := Transaction{}
	err = un.UnmarshalBinary(m)
	if err != nil {
		t.Error("error unmarshalling transaction")
	}
	if tx.Data != un.Data {
		t.Error("incorrect unmarshaled Data")
	}
	if tx.checksum != un.checksum {
		t.Error("incorrect unmarshaled checksum")
	}
}

// TestUnmarshalFails tests the two failure cases of Unmarshal.
func TestUnmarshalFails(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d)
	m, err := tx.MarshalBinary()
	un := Transaction{}
	err = un.UnmarshalBinary(m[0:TxSize-1])
	_, isLen := err.(DataSizeError)
	if !isLen || err.Error() != "incorrect data size given to unmarshaler" {
		t.Error("unmarshal did not report wrong length error")
	}
	zeros := [20]byte{0}
	copy(m[0:20], zeros[:]) // zero out the first 20 bytes
	err = un.UnmarshalBinary(m)
	_, isCS := err.(ChecksumError)
	if !isCS || err.Error() != "incorrect transaction checksum" {
		t.Error("unmarshal did not report checksum error")
	}
}
