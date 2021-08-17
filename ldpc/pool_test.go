package ldpc

import (
	"math"
	"math/rand"
	"testing"
)

func setupData(n int) *PeerSyncState {
	p := &PeerSyncState{
		SyncClock: &SyncClock{
			TransactionTimeout: MaxTimestamp,
			CodewordTimeout:    MaxTimestamp,
			Seq:                1,
		},
	}
	for i := 0; i < n; i++ {
		d := [TxDataSize]byte{}
		rand.Read(d[:])
		ht := NewHashedTransaction(NewTransaction(d, 1))
		p.addTransaction(&ht, MaxTimestamp)
	}
	return p
}

func BenchmarkProduceCodeword(b *testing.B) {
	b.SetBytes(TxSize)
	p := setupData(15000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProduceCodeword(rand.Uint64(), math.MaxUint64/100, rand.Intn(MaxHashIdx), math.MaxUint64)
	}
}

// TestExists tests if the Exists method is correct.
func TestExists(t *testing.T) {
	p := setupData(3)
	// pick one transaction from the pool
	var there Transaction
	for j := 0; j < numBuckets; j++ {
		for _, v := range p.transactionTrie.buckets[0][j].items {
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
	tx := NewHashedTransaction(NewTransaction(d, 1))

	// send to ourself two codewords, one with threshold close to 0, one with threshold=maxuint
	cw0 := p.ProduceCodeword(0, 0, 0, math.MaxUint64)
	cwm := p.ProduceCodeword(0, math.MaxUint64, 0, math.MaxUint64)
	p.InputCodeword(cw0)
	p.InputCodeword(cwm)

	p.addTransaction(&tx, MaxTimestamp)

	// now, cw0 should be untouched
	if p.codewords[0].symbol != cw0.symbol || p.codewords[0].counter != cw0.counter {
		t.Error("AddTransaction touch codewords that it should not change")
	}
	// cwm should be updated
	var shouldbe [TxSize]byte
	copy(shouldbe[:], cwm.symbol[:])
	shouldbeCounter := cwm.counter
	for j := 0; j < numBuckets; j++ {
		for _, v := range p.transactionTrie.buckets[0][j].items {
			shouldbeCounter -= 1
			m, _ := v.Transaction.MarshalBinary()
			for i := 0; i < TxSize; i++ {
				shouldbe[i] ^= m[i]
			}
		}
	}
	if p.codewords[1].symbol != shouldbe {
		t.Error("AddTransaction not updating symbol")
	}
	if p.codewords[1].counter != shouldbeCounter {
		t.Error("AddTransaction not updating counter")
	}
	// tx should be in the pool
	if !p.Exists(tx.Transaction) {
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
	if len(p.codewords) != 1 {
		t.Error("pool contains", len(p.codewords), "codewords, should be 1")
	}
	if p.codewords[0].counter != 0 {
		t.Error("codeword contains", p.codewords[0].counter, "transactions, should be 0")
	}
	if p.codewords[0].symbol != emptySymbol {
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
	for j := 0; j < numBuckets; j++ {
		for _, v := range s1.transactionTrie.buckets[0][j].items {
			tx := v.Transaction
			if count >= s1.transactionTrie.counter-1 {
				missing = tx
				break
			} else {
				ht := NewHashedTransaction(tx)
				s2.addTransaction(&ht, MaxTimestamp)
				count += 1
			}
		}
	}
	c := s1.ProduceCodeword(0, math.MaxUint64, 0, math.MaxUint64) // we want the codeword to cover all elements
	if c.counter != s1.transactionTrie.counter {
		t.Fatal("codeword contains", c.counter, "elements, not equal to", s1.transactionTrie.counter)
	}
	s2.InputCodeword(c)
	s2.tryDecode(nil)
	if s2.transactionTrie.counter != s1.transactionTrie.counter {
		t.Error("pool 2 contains", s2.transactionTrie.counter, "transactions, less than pool 1")
	}
	if len(s2.codewords) != 0 {
		t.Error("pool 2 contains", len(s2.codewords), "codewords, not zero")
	}
	if !s2.Exists(missing) {
		t.Error("cannot find the missing transaction in pool 2")
	}
}
