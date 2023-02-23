package lt

import (
	"testing"
	"bytes"
	"github.com/yangl1996/soliton"
	"math/rand"
	"fmt"
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

func (cw *testCodewordState) intoPendingCodeword() *PendingCodeword[*simpleData] {
	return (*PendingCodeword[*simpleData])(cw)
}

func TestMarkDecodedAndPeelTransaction(t *testing.T) {
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

func TestFailToDecode(t *testing.T) {
	// tx1 is blocking cw1, 2
	tx1 := newTestTransactionState(1)
	var cw1, cw2 *testCodewordState
	cw1 = cw1.xor(tx1) 
	cw2 = cw2.xor(tx1)
	_, _, txFailed := cw1.intoPendingCodeword().failToDecode()
	if txFailed {
		t.Error("incorrectly reporting the pending transaction can be freed")
	}
	if len(tx1.pendingTransaction.blocking) != 1 {
		t.Error("incorrect number of blocked codewords")
	}
	if tx1.pendingTransaction.blocking[0] != cw2.intoPendingCodeword() {
		t.Error("incorrect list of blocked codewords")
	}

	// tx2 is blocking cw3 only
	tx2 := newTestTransactionState(2)
	var cw3 *testCodewordState
	cw3 = cw3.xor(tx2)
	saltedHash, _, txFailed := cw3.intoPendingCodeword().failToDecode()
	if !txFailed {
		t.Error("not reporting the pending transaction can be freed")
	}
	if saltedHash != tx2.saltedHash {
		t.Error("incorrect salted hash of freeable transaction")
	}
}

func BenchmarkDecode(b *testing.B) {
	ks := []int{500, 1000, 2000}
	genrun := func(k int) func(b *testing.B) {
		return func(b *testing.B) {
			dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), uint64(k), 0.03, 0.5)
			e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist, k)
			for i := 0; i < k; i++ {
				tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
				e.AddTransaction(tx)
			}
			cws := []Codeword[*simpleData]{}
			// ensure there are enough codewords for decoding
			for i := 0; i < k*2; i++ {
				cws = append(cws, e.ProduceCodeword())
			}
			
			decs := []*Decoder[*simpleData]{}
			for i := 0; i < b.N; i++ {
				decs = append(decs, NewDecoder[*simpleData](testSalt, k*2))
			}
			b.ReportAllocs()
			b.SetBytes(int64(simpleDataSize * k))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				decoded := 0
				for _, cw := range cws {
					_, txs := decs[i].AddCodeword(cw)
					decoded += len(txs)
					if decoded == k {
						break
					}
				}
			}
		}
	}
	for _, k := range ks {
		b.Run(fmt.Sprintf("k=%d", k), genrun(k))
	}
}
