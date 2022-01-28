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
	hasher.Write(t1[:])
	tx1stub := &pendingTransaction{(uint32)(hasher.Sum64()), []*pendingCodeword{}}
	return tx1, tx1stub
}

func (c *pendingCodeword) addTransaction(t *Transaction, stub *pendingTransaction) {
	c.symbol.XOR(&t.serialized)
	c.members = append(c.members, stub)
	stub.blocking = append(stub.blocking, c)
}

// TestPeelTransaction tests peeling off a transaction from a pending codeword.
func TestPeelTransaction(t *testing.T) {
	tx1, tx1stub := randomTransaction()
	tx2, tx2stub := randomTransaction()

	cw := &pendingCodeword{}
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

	cw1 := &pendingCodeword{}
	cw1.addTransaction(tx1, tx1stub)
	cw1.addTransaction(tx2, tx2stub)

	cw2 := &pendingCodeword{}
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
	//p := newPeer(testSalt)
	// create 
}
