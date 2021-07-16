package ldpc

import (
	"math"
)

// PeerStatus represents the status of a transaction at a peer.
type PeerStatus struct {
	FirstAvailable int
	LastMissing int
}

type TimestampedTransaction struct {
	HashedTransaction
	PeerStatus
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	TransactionId map[Transaction]struct{}
	Transactions []TimestampedTransaction
	Codewords    []PendingCodeword
	ReleasedCodewords []ReleasedCodeword
	Seq          int
}

// NewTransactionPool creates an empty transaction pool.
func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.TransactionId = make(map[Transaction]struct{})
	p.Seq = 1
	return p, nil
}

// Exists checks if a given transaction exists in the pool.
func (p *TransactionPool) Exists(t Transaction) bool {
	_, yes := p.TransactionId[t]
	return yes
}

// AddTransaction adds the transaction into the pool, and searches through all
// released codewords to estimate the time that this transaction is last missing
// from the peer. It assumes that the transaction is never seen at the peer.
// It does nothing if the transaction is already in the pool.
func (p *TransactionPool) AddTransaction(t Transaction) {
	if _, there := p.TransactionId[t]; there {
		return
	}
	tx := WrapTransaction(t)
	ps := PeerStatus{math.MaxInt64, int(t.Timestamp-1)}
	for _, c := range p.ReleasedCodewords {
		// tx cannot be a member of any codeword in ReleasedCodewords
		// otherwise, it is already added before the codeword is
		// released. As a result, we do not bother checking if tx is
		// a member of c.
		if c.Covers(&tx) && c.Seq > ps.LastMissing {
			ps.LastMissing = c.Seq
		}
	}
	for cidx, c := range p.Codewords {
		if c.Covers(&tx) && c.Seq > ps.LastMissing {
			p.Codewords[cidx].AddCandidate(t)
		}
	}
	p.Transactions = append(p.Transactions, TimestampedTransaction{tx, ps})
	p.TransactionId[t] = struct{}{}
}

// MarkCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *TransactionPool) MarkCodewordReleased(c PendingCodeword) []TimestampedTransaction {
	if !c.IsPure() {
		panic("releasing impure codeword")
	}
	var touched []TimestampedTransaction
	// go through each transaction that we know of, is covered by c,
	// but is not a member
	for tidx, txv := range p.Transactions {
		if c.Covers(&txv.HashedTransaction) {
			if _, there := c.Members[txv.Transaction]; there {
				if c.Seq < txv.FirstAvailable {
					txv.FirstAvailable = c.Seq
					p.Transactions[tidx].PeerStatus = txv.PeerStatus
					touched = append(touched, txv)
				}
			} else {
				if c.Seq > txv.LastMissing {
					txv.LastMissing = c.Seq
					p.Transactions[tidx].PeerStatus = txv.PeerStatus
					touched = append(touched, txv)
				}
			}
		}
	}
	r := NewReleasedCodeword(c)
	p.ReleasedCodewords = append(p.ReleasedCodewords, r)
	return touched
}

// InputCodeword takes a new codeword, peels transactions that we are sure is a member of
// it, and stores it.
func (p *TransactionPool) InputCodeword(c Codeword) {
	cw := NewPendingCodeword(c)
	for _, txv := range p.Transactions {
		if cw.Covers(&txv.HashedTransaction) {
			if txv.FirstAvailable <= cw.Seq {
				cw.PeelTransaction(txv.Transaction)
			} else if txv.LastMissing < cw.Seq {
				cw.AddCandidate(txv.Transaction)
			}
		}
	}
	p.Codewords = append(p.Codewords, cw)
}

// TryDecode recursively peels transactions that we know are members of some codewords,
// and puts decoded transactions into the pool.
func (p *TransactionPool) TryDecode() {
	// scan through the codewords to find ones with counter=1
	for cidx, c := range p.Codewords {
		switch c.Counter {
		case 1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				// store the transaction and peel the c/w, so the c/w is pure
				p.AddTransaction(*tx)
				p.Codewords[cidx].PeelTransaction(*tx)
			}
		case -1:
			tx := &Transaction{}
			err := tx.UnmarshalBinary(c.Symbol[:])
			if err == nil {
				// we found something missing from the peer so we do a quick
				// sanity check to see if it exists in our tx set (otherwise
				// how woudl this tx got peeled off the codeword?!)
				if _, there := p.TransactionId[*tx]; !there {
					panic("corrupted codeword counter")
				}
				if _, there := c.Members[*tx]; !there {
					panic("corrupted codeword")
				}
				p.Codewords[cidx].UnpeelTransaction(*tx)
			}
		}
	}
	for cidx, _ := range p.Codewords {
		// try to speculatively peel
		// no worry that we are doing redundant work by scanning through all
		// codewords: SpeculatePeel is smart enough to do nothing if nothing
		// has changed
		tx, ok := p.Codewords[cidx].SpeculatePeel()
		if ok {
			p.AddTransaction(tx)
			p.Codewords[cidx].PeelTransaction(tx)
		}
	}
	// release codewords and update transaction availability estimation
	codes := []PendingCodeword{}	// remaining codewords after this iteration
	updatedTx := []TimestampedTransaction{}
	for _, c := range p.Codewords {
		if c.IsPure() {
			updated := p.MarkCodewordReleased(c)
			updatedTx = append(updatedTx, updated...)
		} else {
			codes = append(codes, c)
		}
	}
	change := false
	// try peel the touched transactions off the codewords
	for cidx, c := range codes {
		for _, txv := range updatedTx {
			if c.Covers(&txv.HashedTransaction) {
				_, there := c.Members[txv.Transaction]
				if !there && c.Seq >= txv.FirstAvailable {
					codes[cidx].PeelTransaction(txv.Transaction)
					change = true
				} else if there && c.Seq <= txv.LastMissing {
					codes[cidx].UnpeelTransaction(txv.Transaction)
					change = true
				}
				if c.Seq < txv.FirstAvailable && c.Seq >= txv.LastMissing {
					newcc := codes[cidx].AddCandidate(txv.Transaction)
					if newcc {
						change = true
					}
				} else {
					newcc := codes[cidx].RemoveCandidate(txv.Transaction)
					if newcc {
						change = true
					}
				}
			}
		}
	}
	p.Codewords = codes
	// if any codeword is updated, then we may decode and release more
	if change {
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
	for _, v := range p.Transactions {
		if cw.Covers(&v.HashedTransaction) && int(v.Timestamp) <= cw.Seq {
			cw.ApplyTransaction(&v.Transaction, Into)
		}
	}
	return cw
}
