package ldpc

import (
	"github.com/dchest/siphash"
	//	"math/rand"
	"hash"
	"testing"
)

var testSalt = [SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
var hasher hash.Hash64 = siphash.New(testSalt[:])
var zeroTx TransactionData = TransactionData{}

func randomTransaction() (*Transaction, *pendingTransaction) {
	t1 := randomBytes()
	tx1 := &Transaction{}
	tx1.UnmarshalBinary(t1[:])
	hasher.Reset()
	hasher.Write(tx1.hash[:])
	tx1stub := &pendingTransaction{(uint32)(hasher.Sum64()), []*PendingCodeword{}}
	return tx1, tx1stub
}

func (c *PendingCodeword) addTransaction(t *Transaction, stub *pendingTransaction) {
	c.symbol.XOR(&t.serialized)
	c.members = append(c.members, stub)
	stub.blocking = append(stub.blocking, c)
}

// TestPeelTransaction tests peeling off a transaction from a pending codeword.
func TestPeelTransaction(t *testing.T) {
	tx1, tx1stub := randomTransaction()
	tx2, tx2stub := randomTransaction()

	cw := &PendingCodeword{}
	cw.addTransaction(tx1, tx1stub)
	cw.addTransaction(tx2, tx2stub)

	cw.peelTransaction(tx1stub, tx1)
	if cw.symbol != tx2.serialized {
		t.Error("incorrect result after peeling")
	}
	if len(cw.members) != 1 || cw.members[0] != tx2stub {
		t.Error("incorrect member after peeling")
	}

	cw.peelTransaction(tx2stub, tx2)
	if cw.symbol != zeroTx {
		t.Error("incorrect result after peeling")
	}
	if len(cw.members) != 0 {
		t.Error("incorrect member after peeling")
	}
}

func TestMarkDecoded(t *testing.T) {
	tx1, tx1stub := randomTransaction()
	tx2, tx2stub := randomTransaction()

	cw1 := &PendingCodeword{}
	cw1.addTransaction(tx1, tx1stub)
	cw1.addTransaction(tx2, tx2stub)

	cw2 := &PendingCodeword{}
	cw2.addTransaction(tx1, tx1stub)

	decodable := tx1stub.markDecoded(tx1, nil)
	if len(decodable) != 2 || decodable[0] != cw1 || decodable[1] != cw2 {
		t.Error("incorrect list of decodable codewords")
	}
	if cw2.symbol != zeroTx {
		t.Error("incorrect result after peeling")
	}
	if len(cw2.members) != 0 {
		t.Error("incorrect member after peeling")
	}
	if cw1.symbol != tx2.serialized {
		t.Error("incorrect result after peeling")
	}
	if len(cw1.members) != 1 || cw1.members[0] != tx2stub {
		t.Error("incorrect member after peeling")
	}
	if !cw1.queued || !cw2.queued {
		t.Error("codewords not queued")
	}
}

func TestDecodeCodewords(t *testing.T) {
	p := NewDecoder(testSalt)
	// create the following codewords:
	// cw0 = tx0 + tx1
	// cw1 = tx1 + tx2 + tx3
	// cw2 = tx1 + tx3
	// cw3 = tx0
	// cw4 = tx0 + tx4 + tx5
	txs := make([]*Transaction, 6)
	txstubs := make([]*pendingTransaction, 6)
	for i := range txs {
		txs[i], txstubs[i] = randomTransaction()
		p.pendingTransactions[txstubs[i].saltedHash] = txstubs[i]
	}
	cws := make([]*PendingCodeword, 5)
	for i := range cws {
		cws[i] = &PendingCodeword{}
	}
	cws[0].addTransaction(txs[0], txstubs[0])
	cws[0].addTransaction(txs[1], txstubs[1])
	cws[1].addTransaction(txs[1], txstubs[1])
	cws[1].addTransaction(txs[2], txstubs[2])
	cws[1].addTransaction(txs[3], txstubs[3])
	cws[2].addTransaction(txs[1], txstubs[1])
	cws[2].addTransaction(txs[3], txstubs[3])
	cws[3].addTransaction(txs[0], txstubs[0])
	cws[4].addTransaction(txs[0], txstubs[0])
	cws[4].addTransaction(txs[4], txstubs[4])
	cws[4].addTransaction(txs[5], txstubs[5])

	// now, mark cws[3] as decodable
	cws[3].queued = true
	newtx := p.decodeCodewords([]*PendingCodeword{cws[3]})
	// we should be able to decode txs 0-3, leaving 5 undecoded
	for i := 0; i < 4; i++ {
		if len(cws[i].members) != 0 {
			t.Error("nonempty member set of decoded codeword")
		}
		if cws[i].symbol != zeroTx {
			t.Error("nonzero symbol of decoded codeword")
		}
	}
	if len(cws[4].members) != 2 {
		t.Error("incorrect number of pending transactions")
	}
	crr := TransactionData{}
	crr.XOR(&txs[4].serialized)
	crr.XOR(&txs[5].serialized)
	if cws[4].symbol != crr {
		t.Error("incorrect symbol for pending codeword")
	}

	if p.NumTransactionsReceived() != 4 || len(newtx) != 4 {
		t.Error("incorrect number of decoded transactions")
	}
	for i := 0; i < 4; i++ {
		dec, there := p.receivedTransactions[txstubs[i].saltedHash]
		if !there {
			t.Error("missing decoded transaction")
		}
		if dec.serialized != txs[i].serialized {
			t.Error("incorrect decoded transaction data")
		}
		found := false
		for _, ptr := range newtx {
			if ptr == dec {
				found = true
				break
			}
		}
		if !found {
			t.Error("missing decoded transaction in returned array")
		}
	}
	if len(p.pendingTransactions) != 2 {
		t.Error("incorrect number of pending transactions")
	}
	for i := 4; i < 6; i++ {
		ped, there := p.pendingTransactions[txstubs[i].saltedHash]
		if !there {
			t.Error("missing pending transaction")
		}
		if ped != txstubs[i] {
			t.Error("mismatching pointers")
		}
		if len(ped.blocking) != 1 || ped.blocking[0] != cws[4] {
			t.Error("incorrect blocking set")
		}
	}
}

func TestAddTransaction(t *testing.T) {
	p := NewDecoder(testSalt)
	// create the following codewords:
	// cw0 = tx0 + tx1
	// cw1 = tx1 + tx2 + tx3
	// cw2 = tx1 + tx3
	txs := make([]*Transaction, 4)
	txstubs := make([]*pendingTransaction, 4)
	for i := range txs {
		txs[i], txstubs[i] = randomTransaction()
		p.pendingTransactions[txstubs[i].saltedHash] = txstubs[i]
	}
	cws := make([]*PendingCodeword, 3)
	for i := range cws {
		cws[i] = &PendingCodeword{}
	}
	cws[0].addTransaction(txs[0], txstubs[0])
	cws[0].addTransaction(txs[1], txstubs[1])
	cws[1].addTransaction(txs[1], txstubs[1])
	cws[1].addTransaction(txs[2], txstubs[2])
	cws[1].addTransaction(txs[3], txstubs[3])
	cws[2].addTransaction(txs[1], txstubs[1])
	cws[2].addTransaction(txs[3], txstubs[3])

	// now, add tx[0] and we should be able to decode everything
	newtx := p.AddTransaction(txs[0])
	for i := 0; i < 3; i++ {
		if len(cws[i].members) != 0 {
			t.Error("nonempty member set of decoded codeword")
		}
		if cws[i].symbol != zeroTx {
			t.Error("nonzero symbol of decoded codeword")
		}
	}

	if p.NumTransactionsReceived() != 4 || len(newtx) != 3 {
		t.Error("incorrect number of decoded transactions")
	}
	for i := 0; i < 4; i++ {
		dec, there := p.receivedTransactions[txstubs[i].saltedHash]
		if !there {
			t.Error("missing decoded transaction")
		}
		if dec.serialized != txs[i].serialized {
			t.Error("incorrect decoded transaction data")
		}
		if i != 0 {
			found := false
			for _, ptr := range newtx {
				if ptr == dec {
					found = true
					break
				}
			}
			if !found {
				t.Error("missing decoded transaction in returned array")
			}
		}
	}
	if len(p.pendingTransactions) != 0 {
		t.Error("incorrect number of pending transactions")
	}
}
