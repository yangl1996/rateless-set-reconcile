package ldpc

import (
	"golang.org/x/crypto/blake2b"
)

// HashedTransaction holds the transaction content and its blake2b hash.
type HashedTransaction struct {
	Transaction
	Hash [blake2b.Size256]byte
	Uint uint64
}

func WrapTransaction(t Transaction) HashedTransaction {
	h := t.HashWithSalt(nil)
	tx := HashedTransaction{}
	tx.Transaction = t
	tx.Uint = t.UintWithSalt(nil)
	copy(tx.Hash[:], h[:])
	return tx
}

var emptySymbol = [TxSize]byte{}

// Codeword holds a codeword (symbol), its threshold, and its salt.
type Codeword struct {
	Symbol [TxSize]byte
	HashRange
	Counter int
}

// TransactionPool holds the transactions a node has received and validated.
type TransactionPool struct {
	Transactions map[HashedTransaction]struct{}
	UniqueToUs map[HashedTransaction]struct{}
	Codewords []Codeword
}

func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.Transactions = make(map[HashedTransaction]struct{})
	p.UniqueToUs = make(map[HashedTransaction]struct{})
	return p, nil
}

func (p *TransactionPool) Exists(t Transaction) bool {
	tx := WrapTransaction(t)
	_, yes := p.Transactions[tx]
	return yes
}

// MarkTransactionUnique marks a transaction as unique to us, which causes
// this transaction to be not XOR'ed from future codewords. It also XORs
// the transaction from all existing codewords.
// It returns without changing TransactionPool
// if the transaction is already there.
func (p *TransactionPool) MarkTransactionUnique(t Transaction) {
	tx := WrapTransaction(t)
	if _, there := p.UniqueToUs[tx]; there {
		return
	}
	p.UniqueToUs[tx] = struct{}{}
	// XOR from existing codes
	m, _ := t.MarshalBinary()
	for cidx := range p.Codewords {
		if p.Codewords[cidx].Covers(tx.Uint) {
			for i := 0; i < TxSize; i++ {
				p.Codewords[cidx].Symbol[i] = p.Codewords[cidx].Symbol[i] ^ m[i]
			}
			p.Codewords[cidx].Counter += 1
		}
	}
}

// AddTransaction adds the transaction into the pool, and XORs it from any
// codeword that fits its hash. It returns without changing TransactionPool
// if the transaction is already there.
func (p *TransactionPool) AddTransaction(t Transaction) {
	tx := WrapTransaction(t)
	if _, there := p.Transactions[tx]; there {
		return
	}
	p.Transactions[tx] = struct{}{}
	// XOR from existing codes
	m, _ := t.MarshalBinary()
	// NOTE: if we range a slice by value, we will get a COPY of the element, not a reference
	for cidx := range p.Codewords {
		if p.Codewords[cidx].Covers(tx.Uint) {
			for i := 0; i < TxSize; i++ {
				p.Codewords[cidx].Symbol[i] = p.Codewords[cidx].Symbol[i] ^ m[i]
			}
			p.Codewords[cidx].Counter -= 1
		}
	}
}

// InputCodeword takes an incoming codeword, scans the transactions in the
// pool, and XOR those that fits the codeword into the codeword symbol.
func (p *TransactionPool) InputCodeword(c Codeword) {
	for v, _ := range p.Transactions {
		if _, there := p.UniqueToUs[v]; there {
			continue
		}
		if c.Covers(v.Uint) {
			m, _ := v.MarshalBinary()
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
	decoded := make(map[Transaction]struct{})
	onlyus := make(map[Transaction]struct{})
	codes := []Codeword{}
	// scan through the codewords to find ones with counter=1 or -1
	// and remove those with counter and symbol=0
	for _, c := range p.Codewords {
		switch c.Counter {
		case 1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				decoded[*tx] = struct{}{}
			} else {
				codes = append(codes, c)
			}
		case -1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				onlyus[*tx] = struct{}{}
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
	// add the remaining codes
	p.Codewords = codes
	// add newly decoded transactions
	for t, _ := range decoded {
		p.AddTransaction(t)
	}
	for t, _ := range onlyus {
		p.MarkTransactionUnique(t)
	}
	if len(decoded) > 0 || len(onlyus) > 0 {
		p.TryDecode()
	}
}

// ProduceCodeword selects transactions where the first 8 byte of the hash
// within HashRange specified by start and frac, and XORs the selected
// transactions together.
func (p *TransactionPool) ProduceCodeword(start, frac uint64) Codeword {
	rg := NewHashRange(start, frac)
	res := [TxSize]byte{}
	count := 0
	for v, _ := range p.Transactions {
		if rg.Covers(v.Uint) {
			m, _ := v.MarshalBinary()
			for i := 0; i < TxSize; i++ {
				res[i] = res[i] ^ m[i]
			}
			count += 1
		}
	}
	return Codeword {
		Symbol: res,
		HashRange: rg,
		Counter: count,
	}
}
