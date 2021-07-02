package ldpc

import (
	"testing"
)

// TestSpeculateSuccess tests speculative peeling. We add three transactions into a codeword,
// supply five candidates where two of them are correct members of the codeword, and
// see if we can successfully peel the codeword.
func TestSpeculateSuccess(t *testing.T) {
	c := Codeword{}
	var members []Transaction
	for i := 0; i < 3; i++ {
		d := randomData()
		tx := NewTransaction(d)
		members = append(members, tx)
		c.ApplyTransaction(&tx, Into)
	}
	cw := NewPendingCodeword(c)
	for i := 0; i < 2; i++ {
		cw.AddCandidate(members[i])
	}
	for i := 0; i < 3; i++ {
		d := randomData()
		tx := NewTransaction(d)
		cw.AddCandidate(tx)
	}
	tx, ok := cw.SpeculatePeel()
	if !ok {
		t.Error("failed to speculatively peel")
	}
	if tx != members[2] {
		t.Error("speculative peel returns wrong result")
	}

	// see if we can peel off tx and get a pure codeword
	cw.PeelTransaction(tx)
	if !cw.IsPure() {
		t.Error("codeword not pure")
	}
}

