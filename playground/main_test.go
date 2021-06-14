package playground

import (
	"math/rand"
	"testing"
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
		p.ProduceCodeword(2, 30)
	}
}

