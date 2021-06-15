package playground

import (
	"math/rand"
	"testing"
	"bytes"
)

func setupData(n int) (*TransactionPool, error) {
	p, err := NewTransactionPool()
	if err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		d := [TxSize]byte{}
		rand.Read(d[:])
		p.AddTransaction(d)
	}
	return p, nil
}

func BenchmarkHashing(b *testing.B) {
	p, err := setupData(15000)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProduceCodeword([]byte{2}, 30)
	}
}

// TestLoopback sends codewords back to itself, so the codeword should have
// counter=0 after being received.
func TestLoopback(t *testing.T) {
	p, err := setupData(1000)
	if err != nil {
		t.Fatal(err)
	}
	c := p.ProduceCodeword([]byte{1, 2, 3}, 30)
	t.Log("produced codeword with", c.Counter, "input elements")
	p.InputCodeword(c)
	if len(p.Codewords) != 1 {
		t.Error("pool contains", len(p.Codewords), "codewords, should be 1")
	}
	if p.Codewords[0].Counter != 0 {
		t.Error("codeword contains", p.Codewords[0].Counter, "transactions, should be 0")
	}
	empty := [TxSize]byte{}
	if bytes.Compare(p.Codewords[0].Symbol[:], empty[:]) != 0 {
		t.Error("codeword has nonzero byte remaining")
	}
}
