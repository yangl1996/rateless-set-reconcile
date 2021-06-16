package ldpc

import (
	"math/rand"
	"math"
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

func BenchmarkProduceCodeword(b *testing.B) {
	p, err := setupData(15000)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProduceCodeword([]byte{2}, math.MaxUint64/5)
	}
}

// TestExists tests if the Exists method is correct.
func TestExists(t *testing.T) {
	p, err := setupData(3)
	if err != nil {
		t.Fatal(err)
	}
	d := [TxSize]byte{}
	rand.Read(d[:])
	if !p.Exists(p.Transactions[0].Transaction) {
		t.Error("failed to locate a transaction that exists in the pool")
	}
	if p.Exists(d) {
		t.Error("mistakenly located a transaction that does not exist in the pool")
	}
}

// TestLoopback sends codewords back to itself, so the codeword should have
// counter=0 after being received.
func TestLoopback(t *testing.T) {
	p, err := setupData(1000)
	if err != nil {
		t.Fatal(err)
	}
	c := p.ProduceCodeword([]byte{1, 2, 3}, math.MaxUint64/5)
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

// TestOneOff sets up two sets with just one element missing in the second
// set, and then sends a codeword covering all elements in the first set to
// the second set. It then verifies that the second set can decode the element.
func TestOneoff(t *testing.T) {
	s1, err := setupData(1000)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewTransactionPool()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(s1.Transactions)-1; i++ {
		s2.AddTransaction(s1.Transactions[i].Transaction)
	}
	missing := s1.Transactions[len(s1.Transactions)-2]
	c := s1.ProduceCodeword([]byte{1, 2, 3}, math.MaxUint64) // we want the codeword to cover all elements
	if c.Counter != len(s1.Transactions) {
		t.Fatal("codeword contains", c.Counter, "elements, not equal to", len(s1.Transactions))
	}
	s2.InputCodeword(c)
	s2.TryDecode()
	if len(s2.Transactions) != len(s1.Transactions) {
		t.Error("pool 2 contains", len(s2.Transactions), "transactions, less than pool 1")
	}
	if len(s2.Codewords) != 0 {
		t.Error("pool 2 contains", len(s2.Codewords), "codewords, not zero")
	}
	if !s2.Exists(missing.Transaction) {
		t.Error("cannot find the missing transaction in pool 2")
	}
}