package lt

import (
	"testing"
	"bytes"
)

type testTransactionState struct {
	data *simpleData
	*pendingTransaction[*simpleData]
}

func newTestTransactionState(i uint64) testTransactionState {
	res := testTransactionState{}
	res.data = newSimpleData(i)
	tx := NewTransaction[*simpleData](res.data)
	hasher.Reset()
	hasher.Write(tx.Hash())
	saltedHash := uint32(hasher.Sum64())
	res.pendingTransaction = &pendingTransaction[*simpleData]{saltedHash, nil}
	return res
}

type testCodewordState PendingCodeword[*simpleData]

func (cw *testCodewordState) xor(tx testTransactionState) *testCodewordState {
	if cw == nil {
		cw = &testCodewordState{}
	}
	cw.symbol = cw.symbol.XOR(tx.data)
	cw.members = append(cw.members, tx.pendingTransaction)
	tx.pendingTransaction.blocking = append(tx.pendingTransaction.blocking, (*PendingCodeword[*simpleData])(cw))
	return cw
}

func (cw *testCodewordState) contains(tx testTransactionState) bool {
	for _, v := range cw.members {
		if v == tx.pendingTransaction {
			return true
		}
	}
	return false
}

func testMarkDecodedAndPeelTransaction(t *testing.T) {
	// create transactions tx1, 2, 3
	tx1 := newTestTransactionState(1)
	tx2 := newTestTransactionState(2)
	tx3 := newTestTransactionState(3)
	// create three pending codewords cw1, 2
	var cw1, cw2, cw3 *testCodewordState
	cw1 = cw1.xor(tx1) // cw1 blocked by tx1
	cw2 = cw2.xor(tx1).xor(tx2) // cw2 blocked by tx1, 2
	cw3 = cw3.xor(tx1).xor(tx2).xor(tx3) // cw3 blocked by tx1, 2, 3
	// try peeling
	queued := tx1.markDecoded(tx1.data, nil)
	if len(queued) != 2 {
		t.Error("incorrect number of codewords became decodable")
	}
	if !cw1.queued || !cw2.queued {
		t.Error("decodable codewords are not queued")
	}
	if cw3.queued {
		t.Error("undecodable codewords are queued")
	}
	if cw1.contains(tx1) || cw2.contains(tx1) || cw3.contains(tx1) {
		t.Error("transaction not peeled")
	}
	if !cw2.contains(tx2) || !cw3.contains(tx2) || !cw3.contains(tx3) {
		t.Error("peeled extra transactions")
	}
	shouldBe := (&simpleData{}).XOR(tx2.data)
	if !bytes.Equal(cw2.symbol[:], shouldBe[:]) {
		t.Error("incorrect symbol after peeling")
	}
	shouldBe = (&simpleData{}).XOR(tx2.data).XOR(tx3.data)
	if !bytes.Equal(cw3.symbol[:], shouldBe[:]) {
		t.Error("incorrect symbol after peeling")
	}
}

func testFailToDecode(t *testing.T) {
}
