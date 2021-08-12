package ldpc

import (
	"math"
)

const MaxTimestamp = math.MaxUint64

// peerStatus represents the status of a transaction at a peer.
type peerStatus struct {
	firstAvailable uint64
	lastMissing    uint64
}

type timestampedTransaction struct {
	hashedTransaction
	peerStatus
}

func (t *timestampedTransaction) markSeenAt(s uint64) {
	if t.firstAvailable > s {
		t.firstAvailable = s
	}
	return
}

// TransactionPool implements the rateless syncing algorithm.
type TransactionPool struct {
	TransactionTrie    trie
	Codewords          []pendingCodeword
	releasedCodewords  []releasedCodeword
	Seq                uint64
	TransactionTimeout uint64 // transactions older than Seq-Timeout will be removed
	CodewordTimeout    uint64 // codewords older than this will be removed
}

// Exists checks if a given transaction exists in the pool.
// This method is slow and should only be used in tests.
func (p *TransactionPool) Exists(t Transaction) bool {
	tp := hashedTransaction{
		Transaction: t,
	}
	tp.Transaction.hashWithSaltInto(nil, &tp.hash)
	h := tp.uint(0)
	b := p.TransactionTrie.buckets[0][h/bucketSize]
	for _, v := range b.items {
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
func (p *TransactionPool) AddTransaction(t Transaction, seen uint64) *timestampedTransaction {
	// we ensured there's no duplicate calls to AddTransaction
	tp := &timestampedTransaction{
		hashedTransaction{
			Transaction: t,
		},
		peerStatus{
			seen,
			t.Timestamp - 1,
		},
	}
	tp.Transaction.hashWithSaltInto(nil, &tp.hash)
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
	for i := len(p.releasedCodewords) - 1; i >= 0; i-- {
		// stop the search when the codeword is older than the tx LastMissing
		if p.releasedCodewords[i].timestamp <= tp.lastMissing {
			break
		}
		// stop at the first released code
		if p.releasedCodewords[i].released && p.releasedCodewords[i].covers(&tp.hashedTransaction) {
			// tx cannot be a member of any codeword in ReleasedCodewords
			// otherwise, it is already added before the codeword is
			// released. As a result, we do not bother checking if tx is
			// a member of c. Note that this is true even for multi-peer.
			tp.lastMissing = p.releasedCodewords[i].timestamp
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
		if p.Codewords[cidx].covers(&tp.hashedTransaction) {
			if p.Codewords[cidx].timestamp >= tp.firstAvailable {
				p.Codewords[cidx].peelTransactionNotCandidate(tp)
			} else if p.Codewords[cidx].timestamp > tp.lastMissing {
				p.Codewords[cidx].addCandidate(tp)
			}
		}
	}
	p.TransactionTrie.addTransaction(tp)
	return tp
}

// markCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *TransactionPool) markCodewordReleased(c *pendingCodeword) {
	// Go through candidates of c. There might very well be transactions in
	// TransactionTrie that are covered by c and not members of c, but if
	// they do not appear in c.Candidates, their LastMissing estimation will
	// not be updated by the release of c. As a result, we only need to search
	// in c.Candidates, not in the whole TransactionTrie.
	for _, txv := range c.candidates {
		if c.timestamp > txv.lastMissing {
			txv.lastMissing = c.timestamp
		}
	}
	p.releasedCodewords[c.releasedIdx].released = true
	return
}

// InputCodeword takes a new codeword, peels transactions that we are sure is a member of
// it, and stores it. It also creates a stub in p.ReleasedCodewords and stores in the
// pending codeword the index to the stub.
func (p *TransactionPool) InputCodeword(c Codeword) {
	p.releasedCodewords = append(p.releasedCodewords, releasedCodeword{c.codewordFilter, c.timestamp, false})
	cwIdx := len(p.Codewords)
	if cwIdx < cap(p.Codewords) {
		p.Codewords = p.Codewords[0 : cwIdx+1]
		p.Codewords[cwIdx].Codeword = c
		p.Codewords[cwIdx].candidates = p.Codewords[cwIdx].candidates[0:0]
		p.Codewords[cwIdx].dirty = true
		p.Codewords[cwIdx].releasedIdx = len(p.releasedCodewords) - 1

	} else {
		p.Codewords = append(p.Codewords, pendingCodeword{
			c,
			make([]*timestampedTransaction, 0, c.counter),
			true,
			len(p.releasedCodewords) - 1,
		})
	}
	cw := &p.Codewords[cwIdx]
	bs, be := cw.bucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= numBuckets {
			bi = bidx - numBuckets
		}
		// lazily remove old transactions from the trie
		bucket := &p.TransactionTrie.buckets[cw.hashIdx][bi]
		tidx := 0
		for tidx < len(bucket.items) {
			v := bucket.items[tidx]
			if p.Seq > v.Timestamp && p.Seq-v.Timestamp > p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items = bucket.items[0:newLen]
				continue
			} else if cw.covers(&v.hashedTransaction) {
				if v.firstAvailable <= cw.timestamp {
					cw.peelTransactionNotCandidate(v)
				} else if v.lastMissing < cw.timestamp {
					cw.addCandidate(v)
				}
			}
			tidx += 1
		}
	}
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
			p.Codewords[cidx].scanCandidates()
			if p.Codewords[cidx].counter == 1 {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(p.Codewords[cidx].symbol[:])
				if err == nil {
					// it's possible the decoded transaction is already
					// in the candidate set; in that case, we do not want
					// add the tx into the pool again -- it's already in
					// the pool
					alreadyThere := false
					for nidx, _ := range p.Codewords[cidx].candidates {
						// compare the checksum first to save time
						if p.Codewords[cidx].candidates[nidx].Transaction.checksum == tx.checksum && p.Codewords[cidx].candidates[nidx].Transaction == *tx {
							// it we found the decoded transaction is already in the candidates, we should remove it from the candidates set
							alreadyThere = true
							p.Codewords[cidx].peelTransactionNotCandidate(p.Codewords[cidx].candidates[nidx])
							p.Codewords[cidx].candidates[nidx] = p.Codewords[cidx].candidates[len(p.Codewords[cidx].candidates)-1]
							p.Codewords[cidx].candidates = p.Codewords[cidx].candidates[0 : len(p.Codewords[cidx].candidates)-1]
							break
						}
					}
					if !alreadyThere {
						// store the transaction and peel the c/w, so the c/w is pure
						p.AddTransaction(*tx, p.Codewords[cidx].timestamp)
						// we would need to peel off tp from cidx, but
						// AddTransaction does it for us.
					}
				}
			}
		}
		for cidx, _ := range p.Codewords {
			// try to speculatively peel
			if p.Codewords[cidx].shouldSpeculate() {
				tx, ok := p.Codewords[cidx].speculatePeel()
				if ok {
					p.AddTransaction(tx, p.Codewords[cidx].timestamp)
					// we would need to peel off tp from cidx, but
					// AddTransaction does it for us.
				}
			}
			p.Codewords[cidx].dirty = false
		}
		// release codewords and update transaction availability estimation
		cwIdx := 0 // idx of the cw we are currently working on
		for cwIdx < len(p.Codewords) {
			shouldRemove := false
			if p.Codewords[cwIdx].isPure() {
				shouldRemove = true
				p.markCodewordReleased(&p.Codewords[cwIdx])
			} else if p.Seq > p.Codewords[cwIdx].timestamp && p.Seq-p.Codewords[cwIdx].timestamp > p.CodewordTimeout {
				shouldRemove = true
			}
			if shouldRemove {
				// remove this codeword by moving from the end of the list
				// however, we want to preserve the slice (and the backing
				// array) of the deleted item so that they can be reused
				// later when we put a new codeword there.
				origSlice := p.Codewords[cwIdx].candidates
				p.Codewords[cwIdx] = p.Codewords[len(p.Codewords)-1]
				p.Codewords[len(p.Codewords)-1].candidates = origSlice
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
	rg := newHashRange(start, frac)
	cw := Codeword{}
	cw.hashRange = rg
	cw.hashIdx = idx
	cw.timestamp = p.Seq
	if cw.timestamp >= lookback {
		cw.minTimestamp = cw.timestamp - lookback
	} else {
		cw.minTimestamp = 0
	}
	p.Seq += 1

	// go through the buckets
	bs, be := cw.bucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= numBuckets {
			bi = bidx - numBuckets
		}
		// lazily remove old transactions from the trie
		bucket := &p.TransactionTrie.buckets[cw.hashIdx][bi]
		tidx := 0
		for tidx < len(bucket.items) {
			v := bucket.items[tidx]
			if p.Seq > v.Timestamp && p.Seq-v.Timestamp > p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items = bucket.items[0:newLen]
				continue
			} else if cw.covers(&v.hashedTransaction) && v.Timestamp <= cw.timestamp {
				cw.applyTransaction(&v.Transaction, into)
			}
			tidx += 1
		}
	}
	return cw
}
