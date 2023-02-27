package iblt

import (
	"github.com/dchest/siphash"
	"testing"
)

func BenchmarkSiphash(b *testing.B) {
	data := make([]byte, 8)
	for i := range data {
		data[i] = byte(i % 256)
	}
	b.ReportAllocs()
	b.SetBytes(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		siphash.Hash(3434234, 7656474568, data)
	}
}

func BenchmarkPolyEvaluation(b *testing.B) {
	sum := 76709685745
	data := 347390857
	eval := 65857434
	b.ReportAllocs()
	b.SetBytes(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum = sum * (eval - data)
	}
}

func BenchmarkXOR(b *testing.B) {
	sum := 384579085
	data := 4324623846
	b.ReportAllocs()
	b.SetBytes(8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum = sum ^ data 
	}
}
