package ldpc

import (
	"github.com/dchest/siphash"
//	"math/rand"
	"hash"
	"testing"
)

var hasher hash.Hash64 = siphash.New([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f})

func degreeTwoCodeword() (*Transaction, *pendingTransaction, *Transaction, *pendingTransaction, *pendingCodeword) {
    t1 := randomBytes()
	tx1 := &Transaction{}
	tx1.UnmarshalBinary(t1[:])

	t2 := randomBytes()
	tx2 := &Transaction{}
	tx2.UnmarshalBinary(t2[:])

    c := TransactionData{}
    c.XOR(&t1)
    c.XOR(&t2)

	cw := &pendingCodeword{
		symbol: c,
	}
	hasher.Reset()
	hasher.Write(t1[:])
	tx1stub := &pendingTransaction{(uint32)(hasher.Sum64()), []*pendingCodeword{cw}}
	hasher.Reset()
	hasher.Write(t2[:])
	tx2stub := &pendingTransaction{(uint32)(hasher.Sum64()), []*pendingCodeword{cw}}
	cw.members = []*pendingTransaction{tx2stub, tx1stub}
	return tx1, tx1stub, tx2, tx2stub, cw
}

// TestPeelTransaction tests peeling off a transaction from a pending codeword.
func TestPeelTransaction(t *testing.T) {
	tx1, tx1stub, tx2, tx2stub, cw := degreeTwoCodeword()

	cw.peelTransaction(tx1stub, tx1)
	if cw.symbol != tx2.serialized {
		t.Error("incorrect result after peeling")
	}
	if len(cw.members) != 1 || cw.members[0] != tx2stub {
		t.Error("incorrect member after peeling")
	}

	cw.peelTransaction(tx2stub, tx2)
	zero := TransactionData{}
	if cw.symbol != zero {
		t.Error("incorrect result after peeling")
	}
	if len(cw.members) != 0 {
		t.Error("incorrect member after peeling")
	}
}


