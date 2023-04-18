package microbenchmarks

import (
	"encoding/binary"
	"github.com/yangl1996/rateless-set-reconcile/lt"
)

func GetTransaction(idx uint64) lt.Transaction[Transaction] {
	return lt.NewTransaction[Transaction](Transaction{idx})
}

var TestKey = [lt.SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

type Transaction struct {
    Idx uint64
}

func (d Transaction) XOR(t2 Transaction) Transaction {
    return Transaction{d.Idx ^ t2.Idx}
}

func (d Transaction) Hash() []byte {
    res := make([]byte, 8)
    binary.LittleEndian.PutUint64(res, d.Idx)
    return res
}
