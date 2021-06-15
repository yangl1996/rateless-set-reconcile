package playground

import (
	"golang.org/x/crypto/blake2b"
	"hash"
)

const TxSize = 512

// HashedTransaction holds the transaction content and its blake2b hash.
// For now, the hash is just computed as a future-proof thing.
type HashedTransaction struct {
	Transaction [TxSize]byte
	Hash [blake2b.Size256]byte
}

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	Threshold byte
	Salt []byte
}

// TransactionPool holds the transactions a node has received and validated.
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

// ProduceCodeword selects transactions where the first byte of the hash
// with a give salt is no bigger than frac, and XORs the selected transactions
// together.
// TODO: using salting to intro randomness into the selection process is bad,
// because we cannot precompute the hash. We should come up with some way to
// efficiently extract randomness from the hash itself. There must be enough
// randomness there.
func (p *TransactionPool) ProduceCodeword(salt []byte, frac byte) Codeword {
	res := [TxSize]byte{}
	for _, v := range p.Transactions {
		p.hasher.Reset()
		p.hasher.Write(v.Transaction[:])
		p.hasher.Write(salt[:])
		h := p.hasher.Sum(nil)
		if h[0] <= frac {
			for i := 0; i < TxSize; i++ {
				res[i] = res[i] ^ v.Transaction[i]
			}
		}
	}
	return Codeword {
		Symbol: res,
		Threshold: frac,
		Salt: salt[:],
	}
}
