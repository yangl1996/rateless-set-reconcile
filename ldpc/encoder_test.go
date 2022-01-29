package ldpc

import (
	"github.com/yangl1996/soliton"
	"testing"
	"math/rand"
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
	dec := newPeer(testSalt)
	ncw := 0
	for len(dec.receivedTransactions) < 50 {
		c := e.ProduceCodeword()
		dec.addCodeword(c)
		ncw+=1
	}
	for _, tx := range e.window {
		_, there := dec.receivedTransactions[tx.saltedHash]
		if !there {
			t.Error("missing transaction in the decoder")
		}
	}
	t.Logf("%d codewords until fully decoded", ncw)
}
