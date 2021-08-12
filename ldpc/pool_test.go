package ldpc

import (
	"math"
	"math/rand"
	"testing"
)

func setupData(n int) *TransactionPool {
	p := &TransactionPool{
		TransactionTimeout: MaxTimestamp,
		CodewordTimeout:    MaxTimestamp,
		Seq:                1,
	}
	for i := 0; i < n; i++ {
		d := [TxDataSize]byte{}
		rand.Read(d[:])
		p.AddTransaction(NewTransaction(d, 1), MaxTimestamp)
	}
	return p
}

func BenchmarkProduceCodeword(b *testing.B) {
	b.SetBytes(TxSize)
	p := setupData(15000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProduceCodeword(rand.Uint64(), math.MaxUint64/100, rand.Intn(MaxUintIdx), math.MaxUint64)
	}
}

// TestExists tests if the Exists method is correct.
func TestExists(t *testing.T) {
	p := setupData(3)
	// pick one transaction from the pool
	var there Transaction
	for j := 0; j < NumBuckets; j++ {
		for _, v := range p.TransactionTrie.Buckets[0][j].Items {
			there = v.Transaction
			break
		}
	}
	if !p.Exists(there) {
		t.Error("failed to locate a transaction that exists in the pool")
	}
	d := [TxDataSize]byte{}
	rand.Read(d[:])
	if p.Exists(NewTransaction(d, 1)) {
		t.Error("mistakenly located a transaction that does not exist in the pool")
	}
}

// TestAddTransaction tests the AddTransaction function.
func TestAddTransaction(t *testing.T) {
	t.Skip("test broken with the new decoding technique")
	p := setupData(1)
	// create a random transaction
	d := [TxDataSize]byte{}
	rand.Read(d[:])
	tx := NewTransaction(d, 1)

	// send to ourself two codewords, one with threshold close to 0, one with threshold=maxuint
	cw0 := p.ProduceCodeword(0, 0, 0, math.MaxUint64)
	cwm := p.ProduceCodeword(0, math.MaxUint64, 0, math.MaxUint64)
	p.InputCodeword(cw0)
	p.InputCodeword(cwm)

	p.AddTransaction(tx, MaxTimestamp)

	// now, cw0 should be untouched
	if p.Codewords[0].Symbol != cw0.Symbol || p.Codewords[0].Counter != cw0.Counter {
		t.Error("AddTransaction touch codewords that it should not change")
	}
	// cwm should be updated
	var shouldbe [TxSize]byte
	copy(shouldbe[:], cwm.Symbol[:])
	shouldbeCounter := cwm.Counter
	for j := 0; j < NumBuckets; j++ {
		for _, v := range p.TransactionTrie.Buckets[0][j].Items {
			shouldbeCounter -= 1
			m, _ := v.Transaction.MarshalBinary()
			for i := 0; i < TxSize; i++ {
				shouldbe[i] ^= m[i]
			}
		}
	}
	if p.Codewords[1].Symbol != shouldbe {
		t.Error("AddTransaction not updating symbol")
	}
	if p.Codewords[1].Counter != shouldbeCounter {
		t.Error("AddTransaction not updating counter")
	}
	// tx should be in the pool
	if !p.Exists(tx) {
		t.Error("AddTransaction did not add new transaction to the pool")
	}
}

// TestLoopback sends codewords back to itself, so the codeword should have
// counter=0 after being received.
func TestLoopback(t *testing.T) {
	t.Skip("test broken with the new decoding technique")
	p := setupData(1000)
	c := p.ProduceCodeword(0, math.MaxUint64/5, 0, math.MaxUint64)
	p.InputCodeword(c)
	if len(p.Codewords) != 1 {
		t.Error("pool contains", len(p.Codewords), "codewords, should be 1")
	}
	if p.Codewords[0].Counter != 0 {
		t.Error("codeword contains", p.Codewords[0].Counter, "transactions, should be 0")
	}
	if p.Codewords[0].Symbol != emptySymbol {
		t.Error("codeword has nonzero byte remaining")
	}
}

// TestOneOff sets up two sets with just one element missing in the second
// set, and then sends a codeword covering all elements in the first set to
// the second set. It then verifies that the second set can decode the element.
func TestOneoff(t *testing.T) {
	s1 := setupData(1000)
	s2 := setupData(0)
	count := 0
	var missing Transaction
	for j := 0; j < NumBuckets; j++ {
		for _, v := range s1.TransactionTrie.Buckets[0][j].Items {
			tx := v.Transaction
			if count >= s1.TransactionTrie.Counter-1 {
				missing = tx
				break
			} else {
				s2.AddTransaction(tx, MaxTimestamp)
				count += 1
			}
		}
	}
	c := s1.ProduceCodeword(0, math.MaxUint64, 0, math.MaxUint64) // we want the codeword to cover all elements
	if c.Counter != s1.TransactionTrie.Counter {
		t.Fatal("codeword contains", c.Counter, "elements, not equal to", s1.TransactionTrie.Counter)
	}
	s2.InputCodeword(c)
	s2.TryDecode()
	if s2.TransactionTrie.Counter != s1.TransactionTrie.Counter {
		t.Error("pool 2 contains", s2.TransactionTrie.Counter, "transactions, less than pool 1")
	}
	if len(s2.Codewords) != 0 {
		t.Error("pool 2 contains", len(s2.Codewords), "codewords, not zero")
	}
	if !s2.Exists(missing) {
		t.Error("cannot find the missing transaction in pool 2")
	}
}
