package ldpc

import (
	"github.com/yangl1996/soliton"
	"math/rand"
	"testing"
	"fmt"
)

func TestCodeRate(t *testing.T) {
	for _, tc := range []int{20, 50, 100, 200, 500, 1000, 2000, 10000} {
		tc := tc
		t.Run(fmt.Sprintf("%d", tc), func(t *testing.T) {
			//t.Parallel()
			dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), uint64(tc), 0.03, 0.5)
			e := NewEncoder(testSalt, dist, tc)
			for i := 0; i < tc; i++ {
				tx, _ := randomTransaction()
				e.AddTransaction(tx)
			}
			dec := NewPeer(testSalt)
			ncw := 0
			for len(dec.receivedTransactions) < tc {
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
			t.Logf("%d codewords (%.2f)", ncw, float64(ncw)/float64(tc))
		})
	}
}
