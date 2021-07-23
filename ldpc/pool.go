package ldpc

import (
	"math"
)

// PeerStatus represents the status of a transaction at a peer.
type PeerStatus struct {
	FirstAvailable int
	LastMissing    int
}

type TimestampedTransaction struct {
	HashedTransaction
	PeerStatus
}

func (t *TimestampedTransaction) MarkSeenAt(s int) {
	if t.FirstAvailable > s {
		t.FirstAvailable = s
	}
	return
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	TransactionTrie   Trie
	TransactionId     map[Transaction]*TimestampedTransaction
	Codewords         []PendingCodeword
	ReleasedCodewords []ReleasedCodeword
	Seq               int
}

/* TODO:
I  Append codewords as they arrive into ReleasedCodewords as placeholders,
   mark them as not released yet so that they are not treated as released.
   When creating pending codewords, add the index to the placeholder. When
   the codeword is released, go to the placeholder and update it.
II If we want to set lowerbound on transaction timestamp in codewords, we
   can lazily remove outdated transactions from the trie. No need to actively
   maintain it.
III To further reduce allocations, preallocate into arries.
*/


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
	tp := &TimestampedTransaction{
		HashedTransaction{
			Transaction: t,
		},
		PeerStatus{
			math.MaxInt64,
			int(t.Timestamp - 1),
		},
	}
	tp.Transaction.HashWithSaltInto(nil, &tp.Hash)
	// get a better estimation on the LastMissing timestamp of the tx by
	// looking at  there
	// are two cases:
	// 1. ps.LastMissing >= c.Seq: no need to update anyhow; do not search
	// 2. ps.LastMissing < c.Seq
	// The peer must have not received the transaction at c.Seq. Note that
	// ps.LastMissing is lower-bounded by the transaction timestamp. So the
	// transaction must not be generated after c.Seq.
	//
	// Also, for this line to be triggered, the transaction must be received
	// by us after the codeword is released (because we are digging the
	// released transaction in AddNewTransaction).
	//
	//       -----------------------------------------------> Time
	//           |           |          |          |
	//            - Tx gen    - c.Seq    - c rls    - Tx add
	//
	// So, we search backwards in time, and stop at the first c which misses
	// tx or when we hit tx.Seq
	for i := len(p.ReleasedCodewords)-1; i >= 0; i-- {
		// stop the search when the codeword is older than the tx LastMissing
		if p.ReleasedCodewords[i].Seq <= tp.LastMissing {
			break
		}
		// stop at the first released code
		if p.ReleasedCodewords[i].Released && p.ReleasedCodewords[i].Covers(&tp.HashedTransaction) {
			// tx cannot be a member of any codeword in ReleasedCodewords
			// otherwise, it is already added before the codeword is
			// released. As a result, we do not bother checking if tx is
			// a member of c. Note that this is true even for multi-peer.
			tp.LastMissing = p.ReleasedCodewords[i].Seq
			// we are searching backwards, so we won't get any better
			// (larger in number) estimation of LastMissing; we can
			// stop here
			break

		}
	}
	// now that we get a better bound on ps.LastMissing, add the tx as candidate
	// to codewords after ps.LastMissing
	for cidx, _ := range p.Codewords {
		if p.Codewords[cidx].Covers(&tp.HashedTransaction) && p.Codewords[cidx].Seq > tp.LastMissing {
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
func (p *TransactionPool) MarkCodewordReleased(c *PendingCodeword) {
	// go through each transaction that we know of, is covered by c,
	// but is not a member
	bs, be := c.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, txv := range p.TransactionTrie.Buckets[c.UintIdx][bi].Items {
			if c.Covers(&txv.HashedTransaction) {
				// Technically, we need to check if txv is a member of the codeword
				// because we should only update LastMissing of txv when it is NOT
				// a member of c but is covered by c. However, if txv IS a member,
				// its FirstAvailable must have been updated to c.Seq when we peeled
				// txv off c. As a result, txv.FirstAvailable <= c.Seq if and only
				// if txv is a member of c.
				if txv.FirstAvailable > c.Seq {
					if c.Seq > txv.LastMissing {
						txv.LastMissing = c.Seq
					}
				}
			}
		}
	}
	p.ReleasedCodewords[c.releasedIdx].Released = true
	return
}

// InputCodeword takes a new codeword, peels transactions that we are sure is a member of
// it, and stores it. It also creates a stub in p.ReleasedCodewords and stores in the
// pending codeword the index to the stub.
func (p *TransactionPool) InputCodeword(c Codeword) {
	p.ReleasedCodewords = append(p.ReleasedCodewords, ReleasedCodeword{c.CodewordFilter, c.Seq, false})
	cw := PendingCodeword{c, nil, true, len(p.ReleasedCodewords)-1}
	bs, be := cw.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, txv := range p.TransactionTrie.Buckets[cw.UintIdx][bi].Items {
			if cw.Covers(&txv.HashedTransaction) {
				if txv.FirstAvailable <= cw.Seq {
					cw.PeelTransactionNotCandidate(txv)
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
	change := true
	for change {
		change = false
		// scan through the codewords to find ones with counter=1
		for cidx, _ := range p.Codewords {
			if p.Codewords[cidx].Counter == 1 {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(p.Codewords[cidx].Symbol[:])
				if err == nil {
					// store the transaction and peel the c/w, so the c/w is pure
					tp := p.AddTransaction(*tx)
					// tp cannot be a candidate; otherwise, it should have been
					// peeled in ScanCandidate of the last pass
					p.Codewords[cidx].PeelTransactionNotCandidate(tp)
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
				tp := p.AddTransaction(tx)
				p.Codewords[cidx].PeelTransactionNotCandidate(tp)
			}
		}
		// release codewords and update transaction availability estimation
		cwIdx := 0 // idx of the cw we are currently working on
		for cwIdx < len(p.Codewords) {
			if p.Codewords[cwIdx].IsPure() {
				p.MarkCodewordReleased(&p.Codewords[cwIdx])
				// remove this codeword by moving from the end of the list
				p.Codewords[cwIdx] = p.Codewords[len(p.Codewords)-1]
				p.Codewords = p.Codewords[0 : len(p.Codewords)-1]
				change = true
			} else {
				cwIdx += 1
			}
		}
		// try peel the touched transactions off the codewords
		for cidx, _ := range p.Codewords {
			p.Codewords[cidx].ScanCandidates()
		}
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
	for bidx := bs; bidx <= be; bidx++ {
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
