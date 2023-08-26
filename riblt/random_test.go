package riblt

import (
	"testing"
)

func BenchmarkNextIndex(b *testing.B) {
	s := mapping{234235, 0}
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
		s.nextIndex()
    }
}
