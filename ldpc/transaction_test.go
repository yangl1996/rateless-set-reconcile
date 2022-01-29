package ldpc

import (
	"golang.org/x/crypto/blake2b"
	"math/rand"
	"testing"
)

func randomBytes() TransactionData {
	d := TransactionData{}
	rand.Read(d[:])
	return d
}

// BenchmarkXOR benchmarks XORing transaction data.
func BenchmarkXORTransaction(b *testing.B) {
	t1 := randomBytes()
	t2 := TransactionData{}
	b.ReportAllocs()
	b.SetBytes(TxSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t2.XOR(&t1)
	}
}

// TestXORTransaction tests XORing two transactions.
func TestXORTransaction(t *testing.T) {
	t1 := randomBytes()
	t2 := randomBytes()
	c := TransactionData{}
	c.XOR(&t1)
	if c != t1 {
		t.Error("incorrect bytes after XOR")
	}
	c.XOR(&t2)
	var shouldBe TransactionData
	for i := 0; i < TxSize; i++ {
		shouldBe[i] = t1[i] ^ t2[i]
	}
	if c != shouldBe {
		t.Error("incorrect bytes after XOR")
	}
}

// TestUnmarshalTransaction tests unmarshalling of transaction data.
func TestUnmarshalTransaction(t *testing.T) {
	d := randomBytes()
	tx := &Transaction{}
	err := tx.UnmarshalBinary(d[0 : TxSize-1])
	if _, ok := err.(DataSizeError); !ok {
		t.Error("failed to report data size mismatch")
	}
	err = tx.UnmarshalBinary(d[:])
	if err != nil {
		t.Error("error unmarshalling")
	}
	if tx.serialized != d {
		t.Error("data corrupted during unmarshalling")
	}
	correctHash := blake2b.Sum512(d[:])
	if correctHash != tx.hash {
		t.Error("incorrect hash after unmarshalling")
	}
}
