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
	*hashedTransaction
	peerStatus
}

func (t *timestampedTransaction) markSeenAt(s uint64) {
	if t.firstAvailable > s {
		t.firstAvailable = s
	}
	return
}

type TransactionSync struct {
	peerStates           []PeerSyncState
	newTransactionBuffer []*hashedTransaction
	Seq                  uint64
	TransactionTimeout   uint64
	CodewordTimeout      uint64
}

func (p *TransactionSync) NewPeer() {
	np := PeerSyncState{
		Seq:                p.Seq,
		TransactionTimeout: p.TransactionTimeout,
		CodewordTimeout:    p.CodewordTimeout,
	}
	// if we already have transactions, then we should init the transaction trie with the
	// transactions we already know; we can use any existing peer as the source
	// because all existing peers should have the same number of txs in their transaction
	// tries.
	if len(p.peerStates) > 0 {
		srcBuckets := p.peerStates[0].transactionTrie.buckets[0]
		for bidx := range srcBuckets {
			for _, t := range srcBuckets[bidx].items {
				np.AddTransaction(t.hashedTransaction, MaxTimestamp)
			}
		}
	}
	p.peerStates = append(p.peerStates, np)
}

// PeerSyncState implements the rateless syncing algorithm.
type PeerSyncState struct {
	transactionTrie    trie
	codewords          []pendingCodeword
	releasedCodewords  []releasedCodeword
	Seq                uint64
	TransactionTimeout uint64 // transactions older than Seq-Timeout will be removed
	CodewordTimeout    uint64 // codewords older than this will be removed
}

// NumAddedTransactions returns the number of transactions added to the pool so far.
func (p *PeerSyncState) NumAddedTransactions() int {
	return p.transactionTrie.counter
}

// NumPendingCodewords returns the number of codewords that have not been decoded and
// have not been dropped.
func (p *PeerSyncState) NumPendingCodewords() int {
	return len(p.codewords)
}

// Exists checks if a given transaction exists in the pool.
// This method is slow and should only be used in tests.
func (p *PeerSyncState) Exists(t Transaction) bool {
	tp := hashedTransaction{
		Transaction: t,
	}
	tp.Transaction.hashWithSaltInto(nil, &tp.hash)
	h := tp.uint(0)
	b := p.transactionTrie.buckets[0][h/bucketSize]
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
func (p *PeerSyncState) AddTransaction(t *hashedTransaction, seen uint64) *timestampedTransaction {
	// we ensured there's no duplicate calls to AddTransaction
	tp := &timestampedTransaction{
		t,
		peerStatus{
			seen,
			t.Timestamp - 1,
		},
	}
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
		if p.releasedCodewords[i].released && p.releasedCodewords[i].covers(tp.hashedTransaction) {
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
	for cidx := range p.codewords {
		if p.codewords[cidx].covers(tp.hashedTransaction) {
			if p.codewords[cidx].timestamp >= tp.firstAvailable {
				p.codewords[cidx].peelTransactionNotCandidate(tp)
			} else if p.codewords[cidx].timestamp > tp.lastMissing {
				p.codewords[cidx].addCandidate(tp)
			}
		}
	}
	p.transactionTrie.addTransaction(tp)
	return tp
}

// markCodewordReleased takes a codeword c that is going to be released.
// It updates all known transactions' last missing and first seen timestamps,
// stores c as a ReleasedCodeword, and returns the list of transactions whose
// availability estimation is updated.
func (p *PeerSyncState) markCodewordReleased(c *pendingCodeword) {
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
func (p *PeerSyncState) InputCodeword(c Codeword) {
	p.releasedCodewords = append(p.releasedCodewords, releasedCodeword{c.codewordFilter, c.timestamp, false})
	cwIdx := len(p.codewords)
	if cwIdx < cap(p.codewords) {
		p.codewords = p.codewords[0 : cwIdx+1]
		p.codewords[cwIdx].Codeword = c
		p.codewords[cwIdx].candidates = p.codewords[cwIdx].candidates[0:0]
		p.codewords[cwIdx].dirty = true
		p.codewords[cwIdx].releasedIdx = len(p.releasedCodewords) - 1

	} else {
		p.codewords = append(p.codewords, pendingCodeword{
			c,
			make([]*timestampedTransaction, 0, c.counter),
			true,
			len(p.releasedCodewords) - 1,
		})
	}
	cw := &p.codewords[cwIdx]
	bs, be := cw.bucketIndexRange()
	for bidx := bs; bidx <= be; bidx++ {
		bi := bidx
		if bidx >= numBuckets {
			bi = bidx - numBuckets
		}
		// lazily remove old transactions from the trie
		bucket := &p.transactionTrie.buckets[cw.hashIdx][bi]
		tidx := 0
		for tidx < len(bucket.items) {
			v := bucket.items[tidx]
			if p.Seq > v.Timestamp && p.Seq-v.Timestamp > p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items = bucket.items[0:newLen]
				continue
			} else if cw.covers(v.hashedTransaction) {
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
func (p *PeerSyncState) TryDecode() {
	change := true
	for change {
		change = false
		// scan through the codewords to find ones with counter=1
		for cidx := range p.codewords {
			// clean up the candidates
			p.codewords[cidx].scanCandidates()
			if p.codewords[cidx].counter == 1 {
				tx := &Transaction{}
				err := tx.UnmarshalBinary(p.codewords[cidx].symbol[:])
				if err == nil {
					// it's possible the decoded transaction is already
					// in the candidate set; in that case, we do not want
					// add the tx into the pool again -- it's already in
					// the pool
					alreadyThere := false
					for nidx := range p.codewords[cidx].candidates {
						// compare the checksum first to save time
						if p.codewords[cidx].candidates[nidx].Transaction.checksum == tx.checksum && p.codewords[cidx].candidates[nidx].Transaction == *tx {
							// it we found the decoded transaction is already in the candidates, we should remove it from the candidates set
							alreadyThere = true
							p.codewords[cidx].peelTransactionNotCandidate(p.codewords[cidx].candidates[nidx])
							p.codewords[cidx].candidates[nidx] = p.codewords[cidx].candidates[len(p.codewords[cidx].candidates)-1]
							p.codewords[cidx].candidates = p.codewords[cidx].candidates[0 : len(p.codewords[cidx].candidates)-1]
							break
						}
					}
					if !alreadyThere {
						ht := NewHashedTransaction(*tx)
						// store the transaction and peel the c/w, so the c/w is pure
						p.AddTransaction(&ht, p.codewords[cidx].timestamp)
						// we would need to peel off tp from cidx, but
						// AddTransaction does it for us.
					}
				}
			}
		}
		for cidx := range p.codewords {
			// try to speculatively peel
			if p.codewords[cidx].shouldSpeculate() {
				tx, ok := p.codewords[cidx].speculatePeel()
				if ok {
					ht := NewHashedTransaction(tx)
					p.AddTransaction(&ht, p.codewords[cidx].timestamp)
					// we would need to peel off tp from cidx, but
					// AddTransaction does it for us.
				}
			}
			p.codewords[cidx].dirty = false
		}
		// release codewords and update transaction availability estimation
		cwIdx := 0 // idx of the cw we are currently working on
		for cwIdx < len(p.codewords) {
			shouldRemove := false
			if p.codewords[cwIdx].isPure() {
				shouldRemove = true
				p.markCodewordReleased(&p.codewords[cwIdx])
			} else if p.Seq > p.codewords[cwIdx].timestamp && p.Seq-p.codewords[cwIdx].timestamp > p.CodewordTimeout {
				shouldRemove = true
			}
			if shouldRemove {
				// remove this codeword by moving from the end of the list
				// however, we want to preserve the slice (and the backing
				// array) of the deleted item so that they can be reused
				// later when we put a new codeword there.
				origSlice := p.codewords[cwIdx].candidates
				p.codewords[cwIdx] = p.codewords[len(p.codewords)-1]
				p.codewords[len(p.codewords)-1].candidates = origSlice
				p.codewords = p.codewords[0 : len(p.codewords)-1]
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
func (p *PeerSyncState) ProduceCodeword(start, frac uint64, idx int, lookback uint64) Codeword {
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
		bucket := &p.transactionTrie.buckets[cw.hashIdx][bi]
		tidx := 0
		for tidx < len(bucket.items) {
			v := bucket.items[tidx]
			if p.Seq > v.Timestamp && p.Seq-v.Timestamp > p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items = bucket.items[0:newLen]
				continue
			} else if cw.covers(v.hashedTransaction) && v.Timestamp <= cw.timestamp {
				cw.applyTransaction(&v.Transaction, into)
			}
			tidx += 1
		}
	}
	return cw
}
