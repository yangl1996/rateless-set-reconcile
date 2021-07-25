package ldpc

import (
	"math"
)

const MaxTimestamp = math.MaxUint64

// PeerStatus represents the status of a transaction at a peer.
type PeerStatus struct {
	FirstAvailable uint64
	LastMissing    uint64
}

type TimestampedTransaction struct {
	HashedTransaction
	PeerStatus
}

func (t *TimestampedTransaction) MarkSeenAt(s uint64) {
	if t.FirstAvailable > s {
		t.FirstAvailable = s
	}
	return
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	TransactionTrie   Trie
	Codewords         []PendingCodeword
	ReleasedCodewords []ReleasedCodeword
	Seq               uint64
}

/* TODO:
I  We can lazily remove outdated transactions (which are not going to be
   included in any codeword, ours or peers'), from the trie. No need to actively
   maintain it. Just remove them as we scan the trie buckets.
II To further reduce allocations, preallocate transactions and codewords  into
   arries.
*/


// NewTransactionPool creates an empty transaction pool.
func NewTransactionPool() (*TransactionPool, error) {
	p := &TransactionPool{}
	p.TransactionTrie = Trie{}
	p.Seq = 1
	return p, nil
}

// Exists checks if a given transaction exists in the pool.
// This method is slow and should only be used in tests.
func (p *TransactionPool) Exists(t Transaction) bool {
	tp := HashedTransaction{
		Transaction: t,
	}
	tp.Transaction.HashWithSaltInto(nil, &tp.Hash)
	h := tp.Uint(0)
	b := p.TransactionTrie.Buckets[0][h/BucketSize]
	for _, v := range b.Items {
		if v.Transaction == t {
			return true
		}
	}
	return false
}

// AddTransaction adds the transaction into the pool, and searches through all
// released codewords to estimate the time that this transaction is last missing
// from the peer. It assumes that the transaction is never seen at the peer.
// It does nothing if the transaction is already in the pool.
func (p *TransactionPool) AddTransaction(t Transaction, seen uint64) *TimestampedTransaction {
	// we ensured there's no duplicate calls to AddTransaction
	tp := &TimestampedTransaction{
		HashedTransaction{
			Transaction: t,
		},
		PeerStatus{
			seen,
			t.Timestamp - 1,
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
	// to codewords after ps.LastMissing; or, if tx is determined to be seen before
	// c.Seq, we can directly peel it off.
	for cidx, _ := range p.Codewords {
		if p.Codewords[cidx].Covers(&tp.HashedTransaction) {
			if p.Codewords[cidx].Seq >= tp.FirstAvailable {
				p.Codewords[cidx].PeelTransactionNotCandidate(tp)
			} else if p.Codewords[cidx].Seq > tp.LastMissing {
				p.Codewords[cidx].AddCandidate(tp)
			}
		}
	}
	p.TransactionTrie.AddTransaction(tp)
	return tp
}

// MarkCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *TransactionPool) MarkCodewordReleased(c *PendingCodeword) {
	// Go through candidates of c. There might very well be transactions in
	// TransactionTrie that are covered by c and not members of c, but if
	// they do not appear in c.Candidates, their LastMissing estimation will
	// not be updated by the release of c. As a result, we only need to search
	// in c.Candidates, not in the whole TransactionTrie.
	for _, txv := range c.Candidates {
		if c.Seq > txv.LastMissing {
			txv.LastMissing = c.Seq
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
	cw := PendingCodeword{c, make([]*TimestampedTransaction, 0, c.Counter), true, len(p.ReleasedCodewords)-1}
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
			// clean up the candidates
			p.Codewords[cidx].ScanCandidates()
			if p.Codewords[cidx].Counter == 1 {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(p.Codewords[cidx].Symbol[:])
				if err == nil {
					// it's possible the decoded transaction is already
					// in the candidate set; in that case, we do not want
					// add the tx into the pool again -- it's already in
					// the pool
					alreadyThere := false
					for nidx, _ := range p.Codewords[cidx].Candidates {
						// compare the checksum first to save time
						if p.Codewords[cidx].Candidates[nidx].Transaction.checksum == tx.checksum && p.Codewords[cidx].Candidates[nidx].Transaction == *tx {
							// it we found the decoded transaction is already in the candidates, we should remove it from the candidates set
							alreadyThere = true
							p.Codewords[cidx].PeelTransactionNotCandidate(p.Codewords[cidx].Candidates[nidx])
							p.Codewords[cidx].Candidates[nidx] = p.Codewords[cidx].Candidates[len(p.Codewords[cidx].Candidates)-1]
							p.Codewords[cidx].Candidates = p.Codewords[cidx].Candidates[0:len(p.Codewords[cidx].Candidates)-1]
							break
						}
					}
					if !alreadyThere {
						// store the transaction and peel the c/w, so the c/w is pure
						p.AddTransaction(*tx, p.Codewords[cidx].Seq)
						// we would need to peel off tp from cidx, but
						// AddTransaction does it for us.
					}
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
				p.AddTransaction(tx, p.Codewords[cidx].Seq)
				// we would need to peel off tp from cidx, but
				// AddTransaction does it for us.
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
	}
}

// ProduceCodeword selects transactions where the idx-th 8 byte of the hash
// within HashRange specified by start and frac, and XORs the selected
// transactions together. The smallest timestamp admissible to the codeword
// will be the current sequence minus lookback, or 0.
func (p *TransactionPool) ProduceCodeword(start, frac uint64, idx int, lookback uint64) Codeword {
	rg := NewHashRange(start, frac)
	cw := Codeword{}
	cw.HashRange = rg
	cw.UintIdx = idx
	cw.Seq = p.Seq
	if cw.Seq >= lookback {
		cw.MinTimestamp = cw.Seq - lookback
	} else {
		cw.MinTimestamp = 0
	}
	p.Seq += 1

	// go through the buckets
	bs, be := cw.BucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= NumBuckets {
			bi = bidx - NumBuckets
		}
		for _, v := range p.TransactionTrie.Buckets[cw.UintIdx][bi].Items {
			if cw.Covers(&v.HashedTransaction) && v.Timestamp <= cw.Seq {
				cw.ApplyTransaction(&v.Transaction, Into)
			}
		}
	}
	return cw
}
