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

func TestEdgeSlepianWolf(t *testing.T) {
	step := 0.01
	for tc := 0.9; tc <= 1.0; tc += step {
		tc := tc
		t.Run(fmt.Sprintf("%.2f", tc), func(t *testing.T) {
			overlap := int(tc * 10000)
			dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), uint64(10000), 0.03, 0.5)
			e := NewEncoder(testSalt, dist, 10000)
			dec := NewPeer(testSalt)
			ncw := 0
			for i := 0; i < 10000; i++ {
				tx, _ := randomTransaction()
				e.AddTransaction(tx)
				if i < overlap {
					dec.AddTransaction(tx)
					ncw += 1
				}
			}
			for len(dec.receivedTransactions) < 10000 {
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
			t.Logf("%d codewords (%.2f)", ncw, float64(ncw)/float64(10000))
		})
	}
}
