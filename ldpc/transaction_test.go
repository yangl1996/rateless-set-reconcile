package ldpc

import (
	"hash"
	"bytes"
	"encoding/binary"
	"golang.org/x/crypto/blake2b"
	"math/rand"
	"testing"
)

func randomData() [TxDataSize]byte {
	d := [TxDataSize]byte{}
	rand.Read(d[:])
	return d
}

// TestMarshal tests marshalling and unmarshalling of transaction body.
func TestMarshalBody(t *testing.T) {
	d := randomData()
	tb := TransactionBody{d, 123}
	m, err := tb.MarshalBinary()
	if err != nil {
		t.Error("error marshalling transaction body")
	}
	un := TransactionBody{}
	err = un.UnmarshalBinary(m)
	if err != nil {
		t.Error("error unmarshalling transaction body")
	}
	if un != tb {
		t.Error("transaction body corrupted during marshalling")
	}
}

// TestNewTransaction tests the creation of a transaction.
func TestNewTransaction(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d, 123)
	buf, _ := tx.TransactionBody.MarshalBinary()

	hasher := checksumPool.Get().(hash.Hash)
	defer checksumPool.Put(hasher)
	hasher.Reset()
	hasher.Write(buf[:])
	var h [ChecksumSize]byte
	hasher.Sum(h[0:0])
	if h != tx.checksum {
		t.Error(h)
		t.Error(tx.checksum)
		t.Error("incorrect checksum in created transaction")
	}
	tb := TransactionBody{d, 123}
	if tx.TransactionBody != tb {
		t.Error("corrupted fields in new transaction")
	}
}

// TestHashingAndUint tests the hashing and uint with salt.
func TestHashingAndUint(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d, 123)
	s := []byte{}
	bodyBytes, _ := tx.TransactionBody.MarshalBinary()
	s = append(s, bodyBytes...)
	s = append(s, tx.checksum[:]...)
	s = append(s, 1, 2, 3) // salt
	hash := blake2b.Sum512(s)
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
	tx := NewTransaction(d, 123)
	m, err := tx.MarshalBinary()
	if err != nil {
		t.Error("error marshalling transaction")
	}
	un := Transaction{}
	err = un.UnmarshalBinary(m)
	if err != nil {
		t.Error("error unmarshalling transaction")
	}
	if tx.TransactionBody != un.TransactionBody {
		t.Error("incorrect body data")
	}
	if tx.checksum != un.checksum {
		t.Error("incorrect unmarshaled checksum")
	}
}

// TestUnmarshalFails tests the two failure cases of Unmarshal. Specifically, we
// focus on two scenarios: when the data is simply corrupted, and when the data
// is the XOR of two valid transactions. The latter is to make sure the hash fn
// is not homomorphic to XOR.
func TestUnmarshalFails(t *testing.T) {
	d := randomData()
	tx := NewTransaction(d, 123)
	m, err := tx.MarshalBinary()
	un := Transaction{}
	err = un.UnmarshalBinary(m[0 : TxSize-1])
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
	d1 := randomData()
	tx1 := NewTransaction(d1, 123)
	m1, _:= tx1.MarshalBinary()
	d2 := randomData()
	tx2 := NewTransaction(d2, 456)
	m2, _:= tx2.MarshalBinary()
	dt := make([]byte, TxSize)
	for i := 0; i < TxSize; i++ {
		dt[i] = m1[i] ^ m2[i]
	}
	err = un.UnmarshalBinary(dt)
	_, isCS = err.(ChecksumError)
	if !isCS || err.Error() != "incorrect transaction checksum" {
		t.Error("unmarshal did not report checksum error after xor")
	}
}
