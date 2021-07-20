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
	TransactionTrie Trie
	TransactionId map[Transaction]*TimestampedTransaction
	Codewords    []PendingCodeword
	ReleasedCodewords []ReleasedCodeword
	Seq          int
}

// NewTransactionPool creates an empty transaction pool.
func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.TransactionTrie = Trie{}
	p.TransactionId = make(map[Transaction]*TimestampedTransaction)
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
func (p *TransactionPool) AddTransaction(t Transaction) *TimestampedTransaction {
	if tp, there := p.TransactionId[t]; there {
		return tp
	}
	tx := WrapTransaction(t)
	ps := PeerStatus{math.MaxInt64, int(t.Timestamp-1)}
	tp := &TimestampedTransaction{tx, ps}
	// BUG: we only need to search for codewords with ps.LastMissing < c.Seq
	// 1. ps.LastMissing >= c.Seq: no need to update anyhow; do not search
	// 2. ps.LastMissing < c.Seq
	//   The peer must have not received the transaction at c.Seq. Note that
	//   ps.LastMissing is lower-bounded by the transaction timestamp. So the
	//   transaction must not be generated after c.Seq.
	//
	//   Also, for this line to be triggered, the transaction must be received
	//   by us after the codeword is released (because we are digging the
	//   released transaction in AddNewTransaction).
	//
	//         -----------------------------------------------> Time
	//             |           |          |          |
	//              - Tx gen    - c.Seq    - c rls    - Tx add
	//
	//   So, we search backwards in time, and stop at the first c which misses
	//   tx or when we hit tx.Seq
	//
	/*
	for _, c := range p.ReleasedCodewords {
		// tx cannot be a member of any codeword in ReleasedCodewords
		// otherwise, it is already added before the codeword is
		// released. As a result, we do not bother checking if tx is
		// a member of c. Note that this is true even for multi-peer.
		if c.Covers(&tx) && c.Seq > ps.LastMissing {
			ps.LastMissing = c.Seq
			panic("test")
		}
	}
	*/
	// now that we get a better bound on ps.LastMissing, add the tx as candidate
	// to codewords after ps.LastMissing
	for cidx, c := range p.Codewords {
		if c.Covers(&tx) && c.Seq > ps.LastMissing {
			p.Codewords[cidx].AddCandidate(tp)
		}
	}
	p.TransactionTrie.AddTransaction(tp)
	p.TransactionId[t] = tp
	if p.TransactionTrie.Counter != len(p.TransactionId) {
		panic("mismatch")
	}
	return tp
}

// MarkCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *TransactionPool) MarkCodewordReleased(c PendingCodeword) []*TimestampedTransaction {
	if !c.IsPure() {
		panic("releasing impure codeword")
	}
	var touched []*TimestampedTransaction
	// go through each transaction that we know of, is covered by c,
	// but is not a member
	bs, be := c.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx ++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, txv := range p.TransactionTrie.Buckets[c.UintIdx][bi].Items {
			if c.Covers(&txv.HashedTransaction) {
				// BUG: This can be saved by updating the txv.FirstAvailable
				// when we peeled it. (Codewords have pointers to txs,
				// so they can actually update the time estimations there)
				if _, there := c.Members[txv]; there {
					if c.Seq < txv.FirstAvailable {
						txv.FirstAvailable = c.Seq
						touched = append(touched, txv)
					}
				} else {
					if c.Seq > txv.LastMissing {
						txv.LastMissing = c.Seq
						touched = append(touched, txv)
					}
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
	bs, be := cw.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx ++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, txv := range p.TransactionTrie.Buckets[cw.UintIdx][bi].Items {
			if cw.Covers(&txv.HashedTransaction) {
				if txv.FirstAvailable <= cw.Seq {
					cw.PeelTransaction(txv)
				} else if txv.LastMissing < cw.Seq {
					cw.AddCandidate(txv)
				}
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
				tp := p.AddTransaction(*tx)
				p.Codewords[cidx].PeelTransaction(tp)
			}
			/*
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
			*/
		}
	}
	for cidx, _ := range p.Codewords {
		// try to speculatively peel
		// no worry that we are doing redundant work by scanning through all
		// codewords: SpeculatePeel is smart enough to do nothing if nothing
		// has changed
		tx, ok := p.Codewords[cidx].SpeculatePeel()
		if ok {
			tp := p.AddTransaction(tx)
			p.Codewords[cidx].PeelTransaction(tp)
		}
	}
	// release codewords and update transaction availability estimation
	updatedTx := []*TimestampedTransaction{}
	cwIdx := 0	// idx of the cw we are currently working on 
	change := false
	for cwIdx < len(p.Codewords) {
		c := p.Codewords[cwIdx]
		if c.IsPure() {
			updated := p.MarkCodewordReleased(c)
			updatedTx = append(updatedTx, updated...)
			// remove this codeword by moving from the end of the list
			p.Codewords[cwIdx] = p.Codewords[len(p.Codewords)-1]
			p.Codewords = p.Codewords[0:len(p.Codewords)-1]
			change = true
		} else {
			cwIdx += 1
		}
	}
	// try peel the touched transactions off the codewords
	for cidx, c := range p.Codewords {
		for _, txv := range updatedTx {
			if c.Covers(&txv.HashedTransaction) {
				// BUG: This check can be saved if we know the txv.FirstAvailable
				// before the update. So that we only peel if the c.Seq is smaller
				// than the previous txv.FirstAvailable (so that it was not peeled
				// before), and is no smaller than the current txv.FirstAvailable
				// (so that it is eligible).
				_, there := c.Members[txv]
				if !there && c.Seq >= txv.FirstAvailable {
					p.Codewords[cidx].PeelTransaction(txv)
				} else if there && c.Seq <= txv.LastMissing {
					p.Codewords[cidx].UnpeelTransaction(txv)
				}
				if c.Seq < txv.FirstAvailable && c.Seq > txv.LastMissing {
					p.Codewords[cidx].AddCandidate(txv)
				} else {
					p.Codewords[cidx].RemoveCandidate(txv)
				}
			}
		}
	}
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

	// go through the buckets
	bs, be := cw.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx ++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, v := range p.TransactionTrie.Buckets[cw.UintIdx][bi].Items {
			if cw.Covers(&v.HashedTransaction) && int(v.Timestamp) <= cw.Seq {
				cw.ApplyTransaction(&v.Transaction, Into)
			}
		}
	}
	return cw
}

