package riblt

import (
	"testing"
)

func BenchmarkNextIndex(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
		s := mapping{uint64(i) % 100000, uint64(i) % 99999}
		s.nextIndex()
    }
}
