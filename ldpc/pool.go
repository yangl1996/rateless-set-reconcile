package ldpc

import (
	"math"
)

const MaxTimestamp = math.MaxUint64

// Problem: what if the peer never includes a transaction? If so, the receiver will
// always treat the transaction as potentially in incoming codewords, increasing the
// decoding cost for all future codewords. We did not have this problem before because
// timeouts were based on absolute timestamps.
// 
// Solution 1: time out the transactions locally. After T since we receive a transaction,
// stop testing it against incoming codewords even if it has never appeared in incoming
// codewords. If we time out too soon, affected codewords can never be decoded. The
// worst case is when one peer is too lagged behind such that it always include all
// transactions late. An improvement is to time out relative to when we first send out
// the transaction in our outgoing codewords. We assume that after T since we include
// a transaction in our codewords, the peer must have decoded and obtained it (worst
// case from us). Then, we only need to wait for T after we send the transaction.
//
// Compared to the previous scheme based on absolute timestamps, we have one and exactly
// one more problem to solve: we no longer know whether a peer will ever include a
// transaction. Previously, we can locally tell by looking at the timestamps. However,
// we now can only infer.

// peerStatus represents the status of a transaction at a peer.
type peerStatus struct {
	// time when the peer last omits the transaction from a codeword
	lastMissing    uint64
	// time when the peer first includes the transaction in a codeword
	firstAvailable uint64
	lastInclude uint64
	firstDrop uint64
	timeAdded uint64
}

func (t *peerStatus) markSeenAt(s uint64) {
	if t.firstAvailable > s {
		t.firstAvailable = s
	}
	if t.lastInclude < s {
		t.lastInclude = s
	}
	return
}

func (t *peerStatus) markMissingAt(s uint64) {
	// -----|------------------------|-------------------------|-------------------------|------------------------
	//      | lastMissing            | firstAvailable          | lastInclude             | firstDrop
	// We need to determine if the omission happens before the peer decodes the transaction, or after the peer times
	// out the transaction. The former can be tested by s < t.lastInclude and the latter can be tested by
	// s > t.firstAvailable.
	if s < t.lastInclude && s > t.lastMissing {
		t.lastMissing = s
	} else if s > t.firstAvailable && s < t.firstDrop {
		t.firstDrop = s
	}
}

type timestampedTransaction struct {
	*hashedTransaction
	peerStatus
}

type SyncClock struct {
	Seq                uint64
	TransactionTimeout uint64 // transactions older than Seq-Timeout will be removed
	CodewordTimeout    uint64 // codewords older than this will be removed
}

type TransactionSync struct {
	PeerStates           []PeerSyncState
	newTransactionBuffer []*hashedTransaction
	SyncClock
}

func (p *TransactionSync) AddPeer() {
	np := PeerSyncState{
		SyncClock: &p.SyncClock,
	}
	// if we already have transactions, then we should init the transaction trie with the
	// transactions we already know; we can use any existing peer as the source
	// because all existing peers should have the same number of txs in their transaction
	// tries.
	if len(p.PeerStates) > 0 {
		srcBuckets := p.PeerStates[0].transactionTrie.buckets[0]
		for bidx := range srcBuckets {
			for _, t := range srcBuckets[bidx].items {
				np.addUnseenTransaction(t.hashedTransaction)
			}
		}
	}
	p.PeerStates = append(p.PeerStates, np)
}

func (p *TransactionSync) AddLocalTransaction(t Transaction) {
	ht := NewHashedTransaction(t)
	htp := &ht
	for pidx := range p.PeerStates {
		p.PeerStates[pidx].addUnseenTransaction(htp)
	}
}

func (p *TransactionSync) TryDecode() {
	for {
		updated := false
		for pidx := range p.PeerStates {
			p.newTransactionBuffer = p.newTransactionBuffer[:0]
			p.newTransactionBuffer = p.PeerStates[pidx].tryDecode(p.newTransactionBuffer)
			if len(p.newTransactionBuffer) != 0 {
				updated = true
			}
			for p2idx := range p.PeerStates {
				if pidx != p2idx {
					for _, htp := range p.newTransactionBuffer {
						p.PeerStates[p2idx].addUnseenTransaction(htp)
					}
				}
			}
		}
		if !updated {
			break
		}
	}
}

// PeerSyncState implements the rateless syncing algorithm.
type PeerSyncState struct {
	transactionTrie   trie
	codewords         []pendingCodeword
	releasedCodewords []releasedCodeword
	*SyncClock
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

func (p *PeerSyncState) addSeenTransaction(t *hashedTransaction, seen uint64) {
	tp := &timestampedTransaction{
		t,
		peerStatus{
			lastMissing: 0,
			firstAvailable: MaxTimestamp,
			lastInclude: 0,
			firstDrop: MaxTimestamp,
			timeAdded: p.Seq,
		},
	}
	tp.markSeenAt(seen)
	p.addTransaction(tp)
}

func (p *PeerSyncState) addUnseenTransaction(t *hashedTransaction) {
	tp := &timestampedTransaction{
		t,
		peerStatus{
			lastMissing: 0,
			firstAvailable: MaxTimestamp,
			lastInclude: 0,
			firstDrop: MaxTimestamp,
			timeAdded: p.Seq,
		},
	}
	p.addTransaction(tp)
}

// addTransaction adds the transaction into the pool, and searches through all
// released codewords to estimate the time that this transaction is last missing
// from the peer. It assumes that the transaction is never seen at the peer.
// It does nothing if the transaction is already in the pool.
func (p *PeerSyncState) addTransaction(tp *timestampedTransaction) {
	// we ensured there's no duplicate calls to AddTransaction
	/*
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
	*/
	// now that we get a better bound on ps.LastMissing, add the tx as candidate
	// to codewords after ps.LastMissing; or, if tx is determined to be seen before
	// c.Seq, we can directly peel it off.
	for cidx := range p.codewords {
		if p.codewords[cidx].covers(tp.hashedTransaction) {
			if p.codewords[cidx].timestamp >= tp.firstAvailable && p.codewords[cidx].timestamp <= tp.lastInclude {
				p.codewords[cidx].peelTransactionNotCandidate(tp)
			} else if p.codewords[cidx].timestamp > tp.lastMissing && p.codewords[cidx].timestamp < tp.firstDrop {
				if p.codewords[cidx].members.mayContain(&tp.bloom) {
					p.codewords[cidx].addCandidate(tp)
				}
			}
		}
	}
	p.transactionTrie.addTransaction(tp)
	return
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
		txv.markMissingAt(c.timestamp)
	}
	p.releasedCodewords[c.releasedIdx].released = true
	return
}

// InputCodeword takes a new codeword, peels transactions that we are sure is a member of
// it, and stores it. It also creates a stub in p.ReleasedCodewords and stores in the
// pending codeword the index to the stub.
func (p *PeerSyncState) InputCodeword(c Codeword) {
	p.releasedCodewords = append(p.releasedCodewords, releasedCodeword{c.hashRangeFilter, c.timestamp, false})
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
			if p.Seq > v.firstDrop || p.Seq >= v.timeAdded + p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items[newLen] = nil
				bucket.items = bucket.items[0:newLen]
				continue
			} else if cw.covers(v.hashedTransaction) {
				if v.firstAvailable <= cw.timestamp && v.lastInclude >= cw.timestamp {
					cw.peelTransactionNotCandidate(v)
				} else if v.lastMissing < cw.timestamp && v.firstDrop > cw.timestamp {
					if cw.members.mayContain(&v.bloom) {
						cw.addCandidate(v)
					} else {
						v.markMissingAt(cw.timestamp)
					}
				}
			}
			tidx += 1
		}
	}
}

// tryDecode recursively peels transactions that we know are members of some codewords,
// and puts decoded transactions into the pool. It appends the decoded transactions to
// t and return the result.
func (p *PeerSyncState) tryDecode(t []*hashedTransaction) []*hashedTransaction {
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
						htp := &ht
						// store the transaction and peel the c/w, so the c/w is pure
						p.addSeenTransaction(htp, p.codewords[cidx].timestamp)
						t = append(t, htp)
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
					htp := &ht
					p.addSeenTransaction(htp, p.codewords[cidx].timestamp)
					t = append(t, htp)
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
	return t
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
			if p.Seq > v.firstDrop || p.Seq >= v.timeAdded + p.TransactionTimeout {
				newLen := len(bucket.items) - 1
				bucket.items[tidx] = bucket.items[newLen]
				bucket.items[newLen] = nil
				bucket.items = bucket.items[0:newLen]
				continue
			} else if v.timeAdded + lookback >= cw.timestamp && cw.covers(v.hashedTransaction) && v.firstAvailable == MaxTimestamp {
				cw.applyTransaction(&v.Transaction, into)
				cw.members.add(&v.bloom)
			}
			tidx += 1
		}
	}
	return cw
}
