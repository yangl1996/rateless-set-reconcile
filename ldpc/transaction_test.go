package ldpc

import (
	"math/rand"
	"testing"
	"encoding/binary"
	"golang.org/x/crypto/blake2b"
)

func randomBytes() TransactionData {
	d := TransactionData{}
	rand.Read(d[:])
	return d
}

// BenchmarkXOR benchmarks XORing transaction data.
func BenchmarkXORTransaction(b *testing.B) {
    t1 := randomBytes()
    t2 := TransactionData{}
    b.ReportAllocs()
    b.SetBytes(TxSize)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        t2.XOR(&t1)
    }
}

func TestXORTransaction(t *testing.T) {
    t1 := randomBytes()
	t2 := randomBytes()
    c := TransactionData{}
    c.XOR(&t1)
    if c != t1 {
        t.Error("incorrect bytes after XOR")
    }
    c.XOR(&t2)
    var shouldBe TransactionData
    for i := 0; i < TxSize; i++ {
        shouldBe[i] = t1[i] ^ t2[i]
    }
    if c != shouldBe {
        t.Error("incorrect bytes after XOR")
    }
}


// TestUnmarshalTransaction tests unmarshalling of transaction data.
func TestUnmarshalTransaction(t *testing.T) {
	d := randomBytes()
	tx := &Transaction{}
	err := tx.UnmarshalBinary(d[0:TxSize-1])
	if _, ok := err.(DataSizeError); !ok {
		t.Error("failed to report data size mismatch")
	}
	err = tx.UnmarshalBinary(d[:])
	if err != nil {
		t.Error("error unmarshalling")
	}
	if tx.serialized != d {
		t.Error("data corrupted during unmarshalling")
	}
	correctHash := blake2b.Sum512(d[:])
	if correctHash != tx.hash {
		t.Error("incorrect hash after unmarshalling")
	}
	correctShortHash := binary.BigEndian.Uint64(correctHash[0:8])
	if correctShortHash != tx.shortHash {
		t.Error("incorrect short hash after unmarshalling")
	}
}

func TestTransactionUint64(t *testing.T) {
	tx := &Transaction{}
	tx.shortHash = 0x0001020304050607
	first3 := tx.Uint64(3)
	if first3 != 0x00050607 {
		t.Errorf("incorrect short hash of length 3, should be 0x050607 got %#08x", first3)
	}
	first8 := tx.Uint64(8)
	if first8 != tx.shortHash {
		t.Errorf("incorrect short hash of length 8, should be 0x0001020304050607 got %#08x", first8)
	}
}
