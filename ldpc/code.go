package ldpc

import (
	"golang.org/x/crypto/blake2b"
	"encoding/binary"
	"hash"
	"bytes"
)

// HashedTransaction holds the transaction content and its blake2b hash.
// For now, the hash is just computed as a future-proof thing.
type HashedTransaction struct {
	Transaction [TxSize]byte
	Hash [blake2b.Size256]byte
}

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	Threshold uint64
	Salt []byte
	Counter int
}

// TransactionPool holds the transactions a node has received and validated.
type TransactionPool struct {
	Transactions []HashedTransaction
	Codewords []Codeword
	hasher hash.Hash
}

func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	var err error
	p.hasher, err = blake2b.New256(nil)
	return p, err
}

func (p *TransactionPool) Exists(t [TxSize]byte) bool {
	for _, v := range p.Transactions {
		if bytes.Compare(v.Transaction[:], t[:]) == 0 {
			return true
		}
	}
	return false
}

func (p *TransactionPool) hashWithSalt(salt []byte, data [TxSize]byte) []byte {
	p.hasher.Reset()
	p.hasher.Write(data[:])
	p.hasher.Write(salt[:])
	return p.hasher.Sum(nil)
}

func (p *TransactionPool) uintWithSalt(salt []byte, data [TxSize]byte) uint64 {
	h := p.hashWithSalt(salt, data)
	return binary.LittleEndian.Uint64(h[0:8])
}

// AddTransaction adds the transaction into the pool, and XORs it from any
// codeword that fits its hash.
func (p *TransactionPool) AddTransaction(t [TxSize]byte) {
	h := p.hashWithSalt(nil, t)
	tx := HashedTransaction{}
	copy(tx.Transaction[:], t[:])
	copy(tx.Hash[:], h)
	p.Transactions = append(p.Transactions, tx)
	// XOR from existing codes
	for _, c := range p.Codewords {
		h := p.uintWithSalt(c.Salt, t)
		if h <= c.Threshold {
			for i := 0; i < TxSize; i++ {
				c.Symbol[i] = c.Symbol[i] ^ t[i]
			}
			c.Counter -= 1
		}
	}
}

// InputCodeword takes an incoming codeword, scans the transactions in the
// pool, and XOR those that fits the codeword into the codeword symbol.
func (p *TransactionPool) InputCodeword(c Codeword) {
	for _, v := range p.Transactions {
		h := p.uintWithSalt(c.Salt, v.Transaction)
		if h <= c.Threshold {
			for i := 0; i < TxSize; i++ {
				c.Symbol[i] = c.Symbol[i] ^ v.Transaction[i]
			}
			c.Counter -= 1
		}
	}
	p.Codewords = append(p.Codewords, c)
}

// TryDecode recursively tries to decode any codeword that we have received
// so far, and puts those decoded into the pool.
func (p *TransactionPool) TryDecode() {
	decoded := [][TxSize]byte{}
	codes := []Codeword{}
	// scan through the codewords to find ones with counter=1
	// and removes those with counter <= 0
	for _, c := range p.Codewords {
		if c.Counter == 1 {
			decoded = append(decoded, c.Symbol)
		} else if c.Counter > 1 {
			codes = append(codes, c)
		}
	}
	p.Codewords = codes
	// add newly decoded transactions
	for _, t := range decoded {
		p.AddTransaction(t)
	}
	if len(decoded) > 0 {
		p.TryDecode()
	}
}

// ProduceCodeword selects transactions where the first 8 byte of the hash
// with a give salt is no bigger than frac, and XORs the selected transactions
// together.
// TODO: using salting to intro randomness into the selection process is bad,
// because we cannot precompute the hash. We should come up with some way to
// efficiently extract randomness from the hash itself. There must be enough
// randomness there.
func (p *TransactionPool) ProduceCodeword(salt []byte, frac uint64) Codeword {
	res := [TxSize]byte{}
	count := 0
	for _, v := range p.Transactions {
		h := p.uintWithSalt(salt, v.Transaction)
		if h <= frac {
			for i := 0; i < TxSize; i++ {
				res[i] = res[i] ^ v.Transaction[i]
			}
			count += 1
		}
	}
	return Codeword {
		Symbol: res,
		Threshold: frac,
		Salt: salt[:],
		Counter: count,
	}
}
