package playground

import (
	"golang.org/x/crypto/blake2b"
	"hash"
)

const TxSize = 512

type HashedTransaction struct {
	Transaction [TxSize]byte
	Hash [blake2b.Size256]byte
}

type TransactionPool struct {
	Transactions []HashedTransaction
	hasher hash.Hash
}

func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	var err error
	p.hasher, err = blake2b.New256(nil)
	return p, err
}

func (p *TransactionPool) AddTransaction(t [TxSize]byte) {
	p.hasher.Reset()
	p.hasher.Write(t[:])
	h := p.hasher.Sum(nil)
	tx := HashedTransaction{}
	tx.Transaction = t
	copy(tx.Hash[:], h)
	p.Transactions = append(p.Transactions, tx)
}

// ProduceCodeword selects transactions where the idx-th byte of its hash is
// less than frac, and XORs them together. idx must be an integer in [0, 32)
// and frac must be an integer in [0, 256].
func (p *TransactionPool) ProduceCodeword(idx int, frac int) [TxSize]byte {
	res := [TxSize]byte{}
	for _, v := range p.Transactions {
		if int(v.Hash[idx]) < frac {
			for i := 0; i < TxSize; i++ {
				res[i] = res[i] ^ v.Transaction[i]
			}
		}
	}
	return res
}
