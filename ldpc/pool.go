package ldpc

import (
	"golang.org/x/crypto/blake2b"
	"unsafe"
)

const (
	Unknown = 0
	Exist   = 1
	Missing = 2
)

// PeerStatus represents the status of a transaction at a peer.
type PeerStatus struct {
	Status int
	Seq    int
}

// HashedTransaction holds the transaction content and its blake2b hash.
type HashedTransaction struct {
	Transaction
	Hash [blake2b.Size]byte
}

// Uint converts the idx-th 8-byte value into an unsigned int and returns
// the result.
func (t *HashedTransaction) Uint(idx int) uint64 {
	return *(*uint64)(unsafe.Pointer(&t.Hash[idx*8]))
}

// WrapTransaction computes the hash of the given transaction, and bundles
// the hash and the transaction into a HashedTransaction.
func WrapTransaction(t Transaction) HashedTransaction {
	h := t.HashWithSalt(nil)
	tx := HashedTransaction{}
	tx.Transaction = t
	copy(tx.Hash[:], h[:])
	return tx
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	Transactions map[HashedTransaction]PeerStatus
	Codewords    []Codeword
	Seq          int
}

// NewTransactionPool creates an empty transaction pool.
func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.Transactions = make(map[HashedTransaction]PeerStatus)
	return p, nil
}

// Exists checks if a given transaction exists in the pool.
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
	if s, there := p.Transactions[tx]; there && (s.Status == Missing) {
		return
	}
	s := p.Transactions[tx]
	s.Status = Missing
	// XOR from existing codes
	for cidx := range p.Codewords {
		if p.Codewords[cidx].Covers(&tx) {
			p.Codewords[cidx].ApplyTransaction(&t, Into)
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
	p.Transactions[tx] = PeerStatus{}
	// XOR from existing codes
	// NOTE: if we range a slice by value, we will get a COPY of the element, not a reference
	for cidx := range p.Codewords {
		if p.Codewords[cidx].Covers(&tx) {
			p.Codewords[cidx].ApplyTransaction(&t, From)
		}
	}
}

// InputCodeword takes an incoming codeword, scans the transactions in the
// pool, and XOR those that fits the codeword into the codeword symbol.
func (p *TransactionPool) InputCodeword(c Codeword) {
	for v, s := range p.Transactions {
		if s.Status == Missing {
			continue
		}
		if c.Covers(&v) {
			c.ApplyTransaction(&v.Transaction, From)
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

// ProduceCodeword selects transactions where the idx-th 8 byte of the hash
// within HashRange specified by start and frac, and XORs the selected
// transactions together.
func (p *TransactionPool) ProduceCodeword(start, frac uint64, idx int) Codeword {
	rg := NewHashRange(start, frac)
	cw := Codeword{}
	cw.HashRange = rg
	cw.UintIdx = idx
	cw.Seq = p.Seq
	p.Seq += 1
	for v, _ := range p.Transactions {
		if cw.Covers(&v) {
			cw.ApplyTransaction(&v.Transaction, Into)
		}
	}
	return cw
}
