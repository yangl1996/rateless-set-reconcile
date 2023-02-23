package lt

import (
	"bytes"
	"fmt"
	"github.com/yangl1996/soliton"
	"math/rand"
	"testing"
)

func TestReset(t *testing.T) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 500, 0.03, 0.5)
	dist2 := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 600, 0.03, 0.5)
	e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist1, 500)
	for i := 0; i < 500; i++ {
		tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
		e.AddTransaction(tx)
	}
	e.Reset(dist2, 600)
	if e.degreeDist != dist2 {
		t.Error("incorrect degree distribution after resetting")
	}
	if len(e.window) != 0 {
		t.Error("window not cleared after resetting")
	}
	if e.windowSize != 600 {
		t.Error("incorrect window size after resetting")
	}
	if len(e.hashes) != 0 {
		t.Error("existing transaction hashes not cleared after resetting")
	}
}

func TestAddTransaction(t *testing.T) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist, 50)
	tx := NewTransaction[*simpleData](newSimpleData(uint64(1)))
	if !e.AddTransaction(tx) {
		t.Error("failed to add transaction when there is no conflict")
	}
	hasher.Reset()
	hasher.Write(tx.Hash())
	saltedHash := uint32(hasher.Sum64())
	if len(e.window) != 1 {
		t.Error("incorrect window after adding transaction")
	}
	if len(e.hashes) != 1 {
		t.Error("incorrect existing transaction hashes after adding transaction")
	}
	added := e.window[0]
	if added.saltedHash != saltedHash {
		t.Error("added transaction has incorrect salted hash")
	}
	if !bytes.Equal(added.hash, tx.hash) {
		t.Error("added transaction has incorrect hash")
	}
	if !bytes.Equal(added.data[:], tx.data[:]) {
		t.Error("added transaction has incorrect data")
	}
	if _, there := e.hashes[saltedHash]; !there {
		t.Error("salted hash of added transaction not present in existing transaction hashes")
	}

	if e.AddTransaction(tx) {
		t.Error("successfully added duplicated transaction")
	}

	for i := 2; i < 51; i++ {
		tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
		if !e.AddTransaction(tx) {
			t.Error("failed to add transaction when there is no conflict")
		}
	}

	newTx := NewTransaction[*simpleData](newSimpleData(uint64(51)))
	if !e.AddTransaction(newTx) {
		t.Error("failed to add transaction when there is no conflict")
	}
	hasher.Reset()
	hasher.Write(newTx.Hash())
	newSaltedHash := uint32(hasher.Sum64())
	if len(e.window) != 50 {
		t.Error("incorrect size of window after adding transaction and flushing first added transaction")
	}
	if len(e.hashes) != 50 {
		t.Error("incorrect size of existing transaction hashes after adding transaction and flushing first added transaction")
	}
	newAdded := e.window[49]
	if newAdded.saltedHash != newSaltedHash {
		t.Error("newAdded transaction has incorrect salted hash")
	}
	if !bytes.Equal(newAdded.hash, newTx.hash) {
		t.Error("newAdded transaction has incorrect hash")
	}
	if !bytes.Equal(newAdded.data[:], newTx.data[:]) {
		t.Error("newAdded transaction has incorrect data")
	}
	if _, there := e.hashes[newSaltedHash]; !there {
		t.Error("salted hash of newAdded transaction not present in existing transaction hashes")
	}
}

func TestProduceCodeword(t *testing.T) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist, 50)
	for i := 0; i < 50; i++ {
		tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
		e.AddTransaction(tx)
	}
	checkDataUnchanged := func() bool {
		for i := 0; i < 50; i++ {
			tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
			if !bytes.Equal(e.window[i].data[:], tx.data[:]) {
				return false
			}
		}
		return true
	}
	for d := 1; d <= 50; d++ {
		cw := e.produceCodeword(d)
		if !checkDataUnchanged() {
			t.Error("window content changed after producing codeword with degree", d)
		}
		sum := &simpleData{}
		for _, saltedHash := range cw.members {
			found := false
			for _, stx := range e.window {
				if stx.saltedHash == saltedHash {
					found = true
					sum = sum.XOR(stx.data)
				}
			}
			if !found {
				t.Error("member of codeword with degree", d, "not present in encoding window")
			}
		}
		if !bytes.Equal(sum[:], cw.symbol[:]) {
			t.Error("codeword with degree", d, "has incorrect symbol")
		}
	}
}

func BenchmarkAddTransaction(b *testing.B) {
	e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, nil, b.N)
	txs := []Transaction[*simpleData]{}
	for i := 0; i < b.N; i++ {
		tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
		txs = append(txs, tx)
	}
	b.ReportAllocs()
	b.SetBytes(simpleDataSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.AddTransaction(txs[i])
	}
}

func BenchmarkProduceCodeword(b *testing.B) {
	ks := []int{500, 1000, 2000}
	genrun := func(k int) func(b *testing.B) {
		return func(b *testing.B) {
			dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), uint64(k), 0.03, 0.5)
			e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist, k)
			for i := 0; i < k; i++ {
				tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
				e.AddTransaction(tx)
			}
			b.ReportAllocs()
			b.SetBytes(simpleDataSize)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.ProduceCodeword()
			}
		}
	}
	for _, k := range ks {
		b.Run(fmt.Sprintf("k=%d", k), genrun(k))
	}
}
