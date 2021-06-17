package ldpc

import (
	"golang.org/x/crypto/blake2b"
)

// HashedTransaction holds the transaction content and its blake2b hash.
// For now, the hash is just computed as a future-proof thing.
type HashedTransaction struct {
	Transaction
	Hash [blake2b.Size256]byte
}

var emptySymbol = [TxSize]byte{}

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	Threshold uint64
	Salt []byte
	Counter int
}

// TransactionPool holds the transactions a node has received and validated.
type TransactionPool struct {
	Transactions map[HashedTransaction]struct{}
	Codewords []Codeword
}

func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.Transactions = make(map[HashedTransaction]struct{})
	return p, nil
}

func (p *TransactionPool) Exists(t Transaction) bool {
	// TODO: remove this nonsense
	h := t.HashWithSalt(nil)
	tx := HashedTransaction{}
	tx.Transaction = t
	copy(tx.Hash[:], h[:])
	_, yes := p.Transactions[tx]
	return yes
}

// RemoveTransaction removes the transaction from the pool, by XORing it
// back into all codewords we have received.
func (p *TransactionPool) RemoveTransaction(t Transaction) {
}

// AddTransaction adds the transaction into the pool, and XORs it from any
// codeword that fits its hash.
func (p *TransactionPool) AddTransaction(t Transaction) {
	h := t.HashWithSalt(nil)
	tx := HashedTransaction{}
	tx.Transaction = t
	copy(tx.Hash[:], h[:])
	p.Transactions[tx] = struct{}{}
	// XOR from existing codes
	m, _ := t.MarshalBinary()
	for _, c := range p.Codewords {
		h := tx.UintWithSalt(c.Salt)
		if h <= c.Threshold {
			for i := 0; i < TxSize; i++ {
				c.Symbol[i] = c.Symbol[i] ^ m[i]
			}
			c.Counter -= 1
		}
	}
}

// InputCodeword takes an incoming codeword, scans the transactions in the
// pool, and XOR those that fits the codeword into the codeword symbol.
func (p *TransactionPool) InputCodeword(c Codeword) {
	for v, _ := range p.Transactions {
		h := v.UintWithSalt(c.Salt)
		m, _ := v.MarshalBinary()
		if h <= c.Threshold {
			for i := 0; i < TxSize; i++ {
				c.Symbol[i] = c.Symbol[i] ^ m[i]
			}
			c.Counter -= 1
		}
	}
	p.Codewords = append(p.Codewords, c)
}

// TryDecode recursively tries to decode any codeword that we have received
// so far, and puts those decoded into the pool.
func (p *TransactionPool) TryDecode() {
	decoded := []Transaction{}
	onlyus := []Transaction{}
	codes := []Codeword{}
	// scan through the codewords to find ones with counter=1 or -1
	// and remove those with counter and symbol=0
	for _, c := range p.Codewords {
		switch c.Counter {
		case 1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				decoded = append(decoded, *tx)
			} else {
				codes = append(codes, c)
			}
		case -1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				onlyus = append(onlyus, *tx)
			} else {
				codes = append(codes, c)
			}
		case 0:
			if c.Symbol != emptySymbol {
				codes = append(codes, c)
			}
		default:
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
	for v, _ := range p.Transactions {
		h := v.UintWithSalt(salt)
		if h <= frac {
			m, _ := v.MarshalBinary()
			for i := 0; i < TxSize; i++ {
				res[i] = res[i] ^ m[i]
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
