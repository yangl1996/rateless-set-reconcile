package playground

import (
	"math/rand"
	"testing"
)

func setupData(n int) [][TxSize]byte {
	data := make([][TxSize]byte, n)
	for i := 0; i < n; i++ {
		rand.Read(data[i][:])
	}
	return data
}

func BenchmarkHashing(b *testing.B) {
	data := setupData(15000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ProduceCodeword(data, 30)
	}
}

