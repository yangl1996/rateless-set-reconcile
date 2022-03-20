package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"math/rand"
	"math"
)

type uniform struct{}

func (u uniform) Uint64() uint64 {
	return 1
}

type bimodal struct {
	prob1 float64
}

func (b bimodal) Uint64() uint64 {
	if rand.Float64() < b.prob1 {
		return 1
	} else {
		return 2
	}
}

func randomTransaction() *ldpc.Transaction {
	d := ldpc.TransactionData{}
    rand.Read(d[:])
	t := &ldpc.Transaction{}
	t.UnmarshalBinary(d[:])
    return t
}

var testKey = [ldpc.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func testOverlap(N int, commonFrac float64) (Ntx, Ncw int) {
	dist1 := soliton.NewRobustSoliton(rand.New(rand.NewSource(1)), uint64(N), 0.03, 0.5)
	dist2 := soliton.NewRobustSoliton(rand.New(rand.NewSource(2)), uint64(N), 0.03, 0.5)
	e1 := ldpc.NewEncoder(testKey, dist1, N)
	e2 := ldpc.NewEncoder(testKey, dist2, N)
	d := ldpc.NewDecoder(testKey, 2147483647)

	txset := make(map[ldpc.Transaction]struct{})
	nc := int(float64(N) * commonFrac)
    nd := N - nc

	for i := 0; i < nc; i++ {
		tx := randomTransaction()
		txset[*tx] = struct{}{}
		e1.AddTransaction(tx)
		e2.AddTransaction(tx)
	}
	for i := 0; i < nd; i++ {
		tx := randomTransaction()
		txset[*tx] = struct{}{}
		e1.AddTransaction(tx)
		tx = randomTransaction()
		txset[*tx] = struct{}{}
		e2.AddTransaction(tx)
	}
	ntx := len(txset)
	ncw := 0
	for d.NumTransactionsReceived() < N+nd {
		c1 := e1.ProduceCodeword()
		c2 := e2.ProduceCodeword()
		_, newtx := d.AddCodeword(c1)
		for _, tx := range newtx {
			delete(txset, *tx)
		}
		_, newtx = d.AddCodeword(c2)
		for _, tx := range newtx {
			delete(txset, *tx)
		}
		ncw += 2
	}
	return ntx, ncw
}

func testDist(dist ldpc.DegreeDistribution, rate float64) float64{
	e := ldpc.NewEncoder(testKey, dist, 50)
	d := ldpc.NewDecoder(testKey, 1000000)
	for i := 0; i < 50; i++ {
		tx := randomTransaction()
		e.AddTransaction(tx)
	}
	credit := 0.0
	factor := rate
	for i := 0; i < 1000000; i++ {
		tx := randomTransaction()
		e.AddTransaction(tx)
		credit += factor
		for credit > 1.0 {
			credit -= 1.0
			c := e.ProduceCodeword()
			d.AddCodeword(c)
		}
	}
	return float64(d.NumTransactionsReceived()) / float64(1000000)
}

func testUntil(r float64, d ldpc.DegreeDistribution) {
	rate := 1.0
	for {
		f := testDist(d, rate)
		fmt.Printf("%.2f, %.2f\n", rate, f*100)
		if f < r {
			rate += 0.1
		} else {
			break
		}
	}
}

func main() {
	fmt.Println("# overlap  mean inflation   stddev inflation")
	Ns := []int{50, 200, 1000}
	for i, N := range Ns {
		if i != 0 {
			// for gnuplot
			fmt.Println()
			fmt.Println()
		}
		fmt.Printf("\"k=%d\"\n", N)
		for overlap := 0.0; overlap <= 1.01; overlap += 0.05 {
			total := 0.0
			totalSq := 0.0
			ntest := 400
			for i := 0; i < ntest; i++ {
				ntx, ncw := testOverlap(N, overlap)
				rate := float64(ncw) / float64(ntx)
				total += rate
				totalSq += rate * rate
			}
			avg := total / float64(ntest)
			stddev := math.Sqrt(totalSq / float64(ntest) - avg * avg)

			fmt.Printf("%.2f %.2f %.2f\n", overlap, avg, stddev)
		}
	}
}
