package playground

import (
	"math/rand"
	"golang.org/x/crypto/blake2b"
	"testing"
)

func BenchmarkHashing(b *testing.B) {
	// populate with tests with random data
	data := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		data[i] = make([]byte, 512)
		rand.Read(data[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blake2b.Sum256(data[i])
	}
}
