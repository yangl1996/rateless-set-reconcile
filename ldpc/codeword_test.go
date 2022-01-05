package ldpc

import (
	"testing"
)

// BenchmarkXORTransaction benchmarks XORing transaction into codeword.
func BenchmarkXORTransaction(b *testing.B) {
	d := randomBytes()
	t := &Transaction{}
	t.UnmarshalBinary(d[:])
	c := Codeword{}
	b.ReportAllocs()
	b.SetBytes(TxSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.XORWithTransaction(t)
	}
}

func TestXORTransaction(t *testing.T) {
	d := randomBytes()
	t1 := &Transaction{}
	t1.UnmarshalBinary(d[:])
	d = randomBytes()
	t2 := &Transaction{}
	t2.UnmarshalBinary(d[:])
	c := Codeword{}
	c.XORWithTransaction(t1)
	if c.symbol != t1.serialized {
		t.Error("incorrect bytes after XOR")
	}
	c.XORWithTransaction(t2)
	var shouldBe [TxSize]byte
	for i := 0; i < TxSize; i++ {
		shouldBe[i] = t1.serialized[i] ^ t2.serialized[i]
	}
	if c.symbol != shouldBe {
		t.Error("incorrect bytes after XOR")
	}
}
