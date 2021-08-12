package ldpc

import (
	"math"
	"testing"
)

func wrapTransaction(t Transaction) *timestampedTransaction {
	tp := &timestampedTransaction{}
	tp.Transaction = t
	tp.peerStatus = peerStatus{math.MaxInt64, t.Timestamp - 1}
	tp.Transaction.hashWithSaltInto(nil, &tp.hash)
	return tp
}

// prepareCodeword returns a codeword with degree deg and the specified numbers of correct
// and total candidates. If correct+1=deg, it also returns the expected transaction after
// peeing. Otherwise, it returns an empty transaction.
func prepareCodeword(deg, correct, total int) (pendingCodeword, *timestampedTransaction) {
	if correct >= deg {
		panic("correct >= deg")
	}
	c := Codeword{}
	c.timestamp = 10
	var members []*timestampedTransaction
	for i := 0; i < deg; i++ {
		d := randomData()
		tx := wrapTransaction(NewTransaction(d, 1))
		members = append(members, tx)
		c.applyTransaction(&tx.Transaction, into)
	}
	cw := pendingCodeword{c, nil, true, 0}
	for i := 0; i < (total - correct); i++ {
		d := randomData()
		tx := wrapTransaction(NewTransaction(d, 1))
		cw.addCandidate(tx)
	}
	for i := 0; i < correct; i++ {
		cw.addCandidate(members[i])
	}
	if correct+1 == deg {
		return cw, members[len(members)-1]
	} else {
		return cw, nil
	}
}

// BenchmarkApplyTransaction benchmarks ApplyTransaction.
func BenchmarkApplyTransaction(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(TxSize)
	c := Codeword{}
	d := randomData()
	tx := NewTransaction(d, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.applyTransaction(&tx, into)
	}
}

// BenchmarkSpeculate benchmarks the performance of speculative peeling.
func BenchmarkSpeculate(b *testing.B) {
	b.ReportAllocs()
	cws, _ := prepareCodeword(70, 68, 72)
	if !cws.shouldSpeculate() {
		b.Fatal("unable to peel in a benchmark")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cws.dirty = true
		cws.speculatePeel()
	}
}

// TestSpeculateNormal tests speculative peeling. We add three transactions into a codeword,
// supply five candidates where two of them are correct members of the codeword, and
// see if we can successfully peel the codeword. We then test if there are not enough
// correct candidates, and see if peeling fails.
func TestSpeculateNormal(t *testing.T) {
	cw, correct := prepareCodeword(3, 2, 5)
	tx, ok := cw.speculatePeel()
	if !ok {
		t.Error("failed to speculatively peel")
	}
	if tx != correct.Transaction {
		t.Error("speculative peel returns wrong result")
	}
	// see if we can peel off tx and get a pure codeword
	cw.peelTransactionNotCandidate(correct)
	if !cw.isPure() {
		t.Error("codeword not pure")
	}
	// see if candidates are removed
	if len(cw.candidates) != 3 {
		t.Error("candidates not removed")
	}

	cw, _ = prepareCodeword(3, 1, 5)
	_, ok = cw.speculatePeel()
	if ok {
		t.Error("speculative peeling did not fail")
	}
}

// TestSpeculateNotEnoughCandidates tests if speculative peeling fails when there are not
// enough candidates.
func TestSpeculateNotEnoughCandidates(t *testing.T) {
	// first test if peeling will succeed when there are enough candidates
	cw, _ := prepareCodeword(3, 2, 2)
	_, ok := cw.speculatePeel()
	if !ok {
		t.Error("peeling failed when there are enough candidates")
	}

	// then test if peeling fails when there are not
	cw, _ = prepareCodeword(3, 1, 1)
	ok = cw.shouldSpeculate()
	if ok {
		t.Error("peeling did not fail when there are not enough candidates")
	}

}

// TestSpeculateTooLowDegree tests if speculative peeling fails when the degree is too
// low.
func TestSpeculateTooLowDegree(t *testing.T) {
	// first test if a degree=2 codeword peels fine
	cw, _ := prepareCodeword(2, 1, 1)
	_, ok := cw.speculatePeel()
	if !ok {
		t.Error("peeling failed when degree large enough")
	}

	// then test if it fails when degree is less than 2
	cw, _ = prepareCodeword(1, 0, 0)
	ok = cw.shouldSpeculate()
	if ok {
		t.Error("peeling succeeded when degree is only 1")
	}
}

// TestCostTooHigh tests if speculative peeling fails when the cost is too high
func TestCostTooHigh(t *testing.T) {
	// first test when the cost is low
	cw, _ := prepareCodeword(20, 19, 19)
	_, ok := cw.speculatePeel()
	if !ok {
		t.Error("peeling failed when the cost is not high")
	}

	// then test when the cost is high
	cw, _ = prepareCodeword(15, 14, 40)
	ok = cw.shouldSpeculate()
	if ok {
		t.Error("peeling succeeded when cost is very high")
	}
}
