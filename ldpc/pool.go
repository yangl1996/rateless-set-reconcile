package ldpc

import (
	"math"
)

// PeerStatus represents the status of a transaction at a peer.
type PeerStatus struct {
	FirstAvailable int
	LastMissing int
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	Transactions map[HashedTransaction]PeerStatus
	Codewords    []PendingCodeword
	ReleasedCodewords []ReleasedCodeword
	Seq          int
}

// NewTransactionPool creates an empty transaction pool.
func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.Transactions = make(map[HashedTransaction]PeerStatus)
	p.Seq = 1
	return p, nil
}

// Exists checks if a given transaction exists in the pool.
func (p *TransactionPool) Exists(t Transaction) bool {
	tx := WrapTransaction(t)
	_, yes := p.Transactions[tx]
	return yes
}

// AddTransaction adds the transaction into the pool, and searches through all
// released codewords to estimate the time that this transaction is last missing
// from the peer. It assumes that the transaction is never seen at the peer.
// It does nothing if the transaction is already in the pool.
func (p *TransactionPool) AddTransaction(t Transaction) {
	tx := WrapTransaction(t)
	if _, there := p.Transactions[tx]; there {
		return
	}
	ps := PeerStatus{math.MaxInt64, 0}
	for _, c := range p.ReleasedCodewords {
		// tx cannot be a member of any codeword in ReleasedCodewords
		// otherwise, it is already added before the codeword is
		// released. As a result, we do not bother checking if tx is
		// a member of c.
		if c.Covers(&tx) && c.Seq > ps.LastMissing {
			ps.LastMissing = c.Seq
		}
	}
	p.Transactions[tx] = ps
}

// MarkCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *TransactionPool) MarkCodewordReleased(c PendingCodeword) []HashedTransaction {
	if !c.IsPure() {
		panic("releasing impure codeword")
	}
	var touched []HashedTransaction
	// go through each transaction that we know of, is covered by c,
	// but is not a member
	for t, s := range p.Transactions {
		if c.Covers(&t) {
			if _, there := c.Members[t.Transaction]; there {
				if c.Seq < s.FirstAvailable {
					s.FirstAvailable = c.Seq
					p.Transactions[t] = s
					touched = append(touched, t)
				}
			} else {
				if c.Seq > s.LastMissing {
					s.LastMissing = c.Seq
					p.Transactions[t] = s
					touched = append(touched, t)
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
	candidates := []HashedTransaction{}
	for v, s := range p.Transactions {
		if cw.Covers(&v) {
			if s.FirstAvailable <= cw.Seq {
				cw.PeelTransaction(v.Transaction)
			} else if s.LastMissing < cw.Seq {
				// collect candidates for our speculation
				candidates = append(candidates, v)
			}
		}
	}
	if len(candidates)+1 >= cw.Counter && math.Pow(float64(len(candidates)), float64(cw.Counter-1)) <= 200000.0 && cw.Counter >= 2 {
		result := make([]int, cw.Counter-1)
		target := cw.Counter - 1
		var recur func(depth int, start int) bool
		recur = func(depth int, start int) bool {
			if depth == target {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(cw.Symbol[:])
				if err == nil {
					return true
				} else {
					return false
				}
			}
			for i := start; i < len(candidates); i++ {
				cw.PeelTransaction(candidates[i].Transaction)
				result[depth] = i
				ok := recur(depth+1, i+1)
				if ok == true {
					return true
				} else {
					cw.UnpeelTransaction(candidates[i].Transaction)
				}
			}
			return false
		}
		ok := recur(0, 0)
		if ok {
			tx := &Transaction{}
			err := tx.UnmarshalBinary(cw.Symbol[:])
			if err != nil {
				panic("supposed to be okay")
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
				if _, there := p.Transactions[WrapTransaction(*tx)]; !there {
					panic("corrupted codeword counter")
				}
				if _, there := c.Members[*tx]; !there {
					panic("corrupted codeword")
				}
				p.Codewords[cidx].UnpeelTransaction(*tx)
			}
		case 0:
			// try to speculatively fix some codewords
			if !c.IsPure() {
				for t, _ := range p.Codewords[cidx].Members {
					// filter by the availability estimation and only
					// try those that we are unsure
					if p.Transactions[WrapTransaction(t)].FirstAvailable > c.Seq {
						p.Codewords[cidx].UnpeelTransaction(t)
						tx := &Transaction{}
						err := tx.UnmarshalBinary(p.Codewords[cidx].Symbol[:])
						if err == nil {
							p.AddTransaction(*tx)
							p.Codewords[cidx].PeelTransaction(*tx)
							break
						} else {
							p.Codewords[cidx].PeelTransaction(t)
						}
					}
				}
			}
		}
	}
	// release codewords and update transaction availability estimation
	codes := []PendingCodeword{}	// remaining codewords after this iteration
	updatedTx := make(map[HashedTransaction]struct{})
	for _, c := range p.Codewords {
		if c.IsPure() {
			updated := p.MarkCodewordReleased(c)
			for _, t := range updated {
				updatedTx[t] = struct{}{}
			}
		} else {
			codes = append(codes, c)
		}
	}
	change := false
	// try peel the touched transactions off the codewords
	for cidx, c := range codes {
		for t, _ := range updatedTx {
			if c.Covers(&t) {
				_, there := c.Members[t.Transaction]
				if !there && c.Seq > p.Transactions[t].LastMissing {
					// TODO: we are speculating here
					codes[cidx].PeelTransaction(t.Transaction)
					change = true
				} else if there && c.Seq <= p.Transactions[t].LastMissing {
					codes[cidx].UnpeelTransaction(t.Transaction)
					change = true
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
	for v, _ := range p.Transactions {
		if cw.Covers(&v) {
			cw.ApplyTransaction(&v.Transaction, Into)
		}
	}
	return cw
}
