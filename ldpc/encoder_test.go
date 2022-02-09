package ldpc

import (
	"github.com/yangl1996/soliton"
	"math/rand"
	"testing"
)

func BenchmarkProduceCodeword(b *testing.B) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder(testSalt, dist, 50)
	for i := 0; i < 50; i++ {
		tx, _ := randomTransaction()
		e.AddTransaction(tx)
	}
	b.ReportAllocs()
	b.SetBytes(TxSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ProduceCodeword()
	}
}

func BenchmarkAddTransaction(b *testing.B) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder(testSalt, dist, 50)
	for i := 0; i < 50; i++ {
		tx, _ := randomTransaction()
		e.AddTransaction(tx)
	}
	tx, _ := randomTransaction()
	b.ReportAllocs()
	b.SetBytes(TxSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.AddTransaction(tx)
	}
}

func TestEncodeAndDecode(t *testing.T) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder(testSalt, dist, 50)
	for i := 0; i < 50; i++ {
		tx, _ := randomTransaction()
		e.AddTransaction(tx)
	}
	dec := NewDecoder(testSalt)
	ncw := 0
	for len(dec.receivedTransactions) < 50 {
		c := e.ProduceCodeword()
		dec.AddCodeword(c)
		ncw += 1
	}
	for _, tx := range e.window {
		_, there := dec.receivedTransactions[tx.saltedHash]
		if !there {
			t.Error("missing transaction in the decoder")
		}
	}
	t.Logf("%d codewords until fully decoded", ncw)
}

func BenchmarkDecode(b *testing.B) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 50, 0.03, 0.5)
	e := NewEncoder(testSalt, dist, 50)
	txHashes := make(map[uint32]struct{})
	for i := 0; i < 50; i++ {
		tx, stub := randomTransaction()
		txHashes[stub.saltedHash] = struct{}{}
		e.AddTransaction(tx)
	}
	// pre-generate 1.35N codewords for N transactions arrival in uniform pattern
	codewords := make([]*Codeword, 0, b.N+b.N/3)
	credit := 0.0
	for i := 0; i < b.N; i++ {
		tx, stub := randomTransaction()
		txHashes[stub.saltedHash] = struct{}{}
		e.AddTransaction(tx)
		credit += 1.35
		for credit > 1.0 {
			credit -= 1.0
			cw := e.ProduceCodeword()
			codewords = append(codewords, cw)
		}
	}
	dec := NewDecoder(testSalt)
	b.ReportAllocs()
	b.SetBytes(TxSize)
	b.ResetTimer()
	for _, cw := range codewords {
		dec.AddCodeword(cw)
	}
	b.StopTimer()
	ndec := len(dec.receivedTransactions)
	for k, _ := range dec.receivedTransactions {
		_, there := txHashes[k]
		if !there {
			b.Error("decoded transaction", k, "that we did not generate")
		}
	}
	b.Logf("decoded %d out of %d (%.2f%%)", ndec, b.N, float64(ndec)/float64(b.N)*100.0)
}
