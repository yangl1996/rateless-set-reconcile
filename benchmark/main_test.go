package main

import (
	"testing"
	"crypto/sha256"
	"fmt"
)

func BenchmarkRandInt(b *testing.B) {
	lens := []int{4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288, 1048576, 2097152, 4194304, 8388608}
	for _, l := range lens {
		b.Run(fmt.Sprintf("N=%d", l), func(b *testing.B) {
			data := make([]byte, l)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sha256.Sum256(data)
			}
		})
	}
}
