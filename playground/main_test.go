package playground

import (
	"math/rand"
	"golang.org/x/crypto/blake2b"
	"testing"
)

func setupData(n int) [][]byte {
	data := make([][]byte, n)
	for i := 0; i < n; i++ {
		data[i] = make([]byte, 512)
		rand.Read(data[i])
	}
	return data
}

func BenchmarkHashing(b *testing.B) {
	data := setupData(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blake2b.Sum256(data[i])
	}
}

