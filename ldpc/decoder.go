package ldpc

import (
	"github.com/dchest/siphash"
	"hash"
	"sync"
)

const SaltSize = 16

var pendingTransactionPool = sync.Pool{
	New: func() interface{} {
		return &pendingTransaction{}
	},
}

type pendingTransaction struct {
	saltedHash uint32
	blocking   []*PendingCodeword
}

func (tx *pendingTransaction) reset() {
	tx.blocking = tx.blocking[:0]
}

func (tx *pendingTransaction) markDecoded(preimage *Transaction, decodableCws []*PendingCodeword) []*PendingCodeword {
	// peel all codewords containing this transaction
	for idx, peelable := range tx.blocking {
		peelable.peelTransaction(tx, preimage)
		// if it is now decodable, queue it for decoding
		if len(peelable.members) <= 1 && !peelable.queued {
			peelable.queued = true
			decodableCws = append(decodableCws, peelable)
		}
		// set the pointer to nil to free the pointed codeword stub
		tx.blocking[idx] = nil
	}
	pendingTransactionPool.Put(tx)
	return decodableCws
}

var pendingCodewordPool = sync.Pool{
	New: func() interface{} {
		return &PendingCodeword{}
	},
}

type PendingCodeword struct {
	symbol  TransactionData
	members []*pendingTransaction
	queued  bool
	decoded bool
	// TODO: show if the codeword failed to decode
	freed   bool
}

func (cw *PendingCodeword) Free() {
	cw.freed = true
	if cw.decoded {
		pendingCodewordPool.Put(cw)
	}
}

func (cw *PendingCodeword) Decoded() bool {
	return cw.decoded
}

func (cw *PendingCodeword) reset() {
	cw.members = cw.members[:0]
	cw.queued = false
	cw.decoded = false
	cw.freed = false
}

func (cw *PendingCodeword) failToDecode() (uint32, bool) {
	if len(cw.members) != 1 {
		panic("failing a codeword when it has more than 1 members")
	}
	ptr := cw.members[0]
	for cwIdx, cwPtr := range ptr.blocking {
		if cwPtr == cw {
			// zero the pointer so that the pointed tx does not leak
			cw.members[0] = nil
			cw.members = cw.members[:0]
			// remove the link from the pending tx to this failed cw
			l := len(ptr.blocking)
			ptr.blocking[cwIdx] = ptr.blocking[l-1]
			ptr.blocking[l-1] = nil
			ptr.blocking = ptr.blocking[:l-1]
			// free the pending tx if it is blocking nothing
			if len(ptr.blocking) == 0 {
				hash := ptr.saltedHash
				pendingTransactionPool.Put(ptr)
				return hash, true
			} else {
				return 0, false
			}
		}
	}
	panic("unable to find blocked codeword in pending transaction")
}

func (peelable *PendingCodeword) peelTransaction(stub *pendingTransaction, preimage *Transaction) {
	for idx, ptr := range peelable.members {
		if ptr == stub {
			l := len(peelable.members)
			// pop the ptr by swapping with the last item
			// we need to set the deleted ptr to nil to
			// free the pointed value
			peelable.members[idx] = peelable.members[l-1]
			peelable.members[l-1] = nil
			peelable.members = peelable.members[:l-1]
			peelable.symbol.XOR(&preimage.serialized)
			return
		}
	}
	panic("unable to peel decoded transaction from codeword pointing to it")
}

type recentTransaction struct {
	ptr *Transaction
	hash uint32
}

type Decoder struct {
	receivedTransactions map[uint32]*Transaction
	recentTransactions []recentTransaction
	pendingTransactions  map[uint32]*pendingTransaction
	hasher               hash.Hash64
	numTransactionsDecoded int
	numTransactionsMemorized int
}

func NewDecoder(salt [SaltSize]byte) *Decoder {
	p := &Decoder{
		receivedTransactions: make(map[uint32]*Transaction),
		pendingTransactions:  make(map[uint32]*pendingTransaction),
		hasher:               siphash.New(salt[:]),
		numTransactionsMemorized: 262144,
	}
	return p
}

func (p *Decoder) storeNewTransaction(hash uint32, t *Transaction) {
	p.receivedTransactions[hash] = t
	p.recentTransactions = append(p.recentTransactions, recentTransaction{t, hash})
	p.numTransactionsDecoded += 1
	for len(p.recentTransactions) > p.numTransactionsMemorized {
		if e := p.receivedTransactions[p.recentTransactions[0].hash]; e == p.recentTransactions[0].ptr {
			delete(p.receivedTransactions, p.recentTransactions[0].hash)
		}
		p.recentTransactions = p.recentTransactions[1:]
	}
}

func (p *Decoder) AddCodeword(rawCodeword *Codeword) (*PendingCodeword, []*Transaction) {
	cw := pendingCodewordPool.Get().(*PendingCodeword)
	cw.reset()
	cw.symbol = rawCodeword.Symbol
	for _, member := range rawCodeword.Members {
		pending, pendingExists := p.pendingTransactions[member]
		received, receivedExists := p.receivedTransactions[member]
		if !receivedExists {
			if !pendingExists {
				// we have never heard of it
				pending = pendingTransactionPool.Get().(*pendingTransaction)
				pending.reset()
				pending.saltedHash = member
				p.pendingTransactions[member] = pending
			}
			// link to the pending transaction
			pending.blocking = append(pending.blocking, cw)
			cw.members = append(cw.members, pending)
		} else {
			// sanity check
			if !pendingExists {
				// peel the transaction
				cw.symbol.XOR(&received.serialized)
			} else {
				panic("transaction is marked both received and pending")
			}
		}
	}
	if len(cw.members) <= 1 {
		cw.queued = true
		queue := []*PendingCodeword{cw}
		txs := p.decodeCodewords(queue)
		return cw, txs
	}
	return cw, nil
}

func (p *Decoder) AddTransaction(t *Transaction) []*Transaction {
	p.hasher.Reset()
	p.hasher.Write(t.hash[:])
	hash := (uint32)(p.hasher.Sum64())
	if existing, there := p.receivedTransactions[hash]; !there {
		p.storeNewTransaction(hash, t)
		if pending, there := p.pendingTransactions[hash]; there {
			// quick sanity check
			if pending.saltedHash != hash {
				panic("salted hash of retrieved transaction stub does not match the hash computed from the full transaction")
			}
			// peel the transaction and try decoding
			delete(p.pendingTransactions, hash)
			queue := pending.markDecoded(t, nil)
			return p.decodeCodewords(queue)
		} else {
			return nil
		}
	} else {
		if existing.serialized == t.serialized {
			// something that we already know; do not do anything
			return nil
		} else {
			// adding a transaction that is a hash conflict with an existing one that we have not forgotten
			p.storeNewTransaction(hash, t)
			if _, there := p.pendingTransactions[hash]; there {
				panic("pending transaction is already decoded")
			} else {
				return nil
			}
		}
	}
}

// decodeCodewords decodes the list of codewords cws, and returns the list of
// transactions decoded. It updates its local receivedTransactions set.
func (p *Decoder) decodeCodewords(queue []*PendingCodeword) []*Transaction {
	newTx := []*Transaction{}
	for len(queue) > 0 {
		// pop the last item from the queue (stack)
		c := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if !c.queued {
			panic("decoding a codeword not queued")
		}
		if len(c.members) == 0 {
			// nothing to do, already fully peeled
		} else if len(c.members) == 1 {
			// we can decode and obtain one transaction; first check if it is
			// already decoded
			tx := c.members[0]
			// TODO: the following two checks are just for sanity and are potentially
			// costly
			if _, there := p.receivedTransactions[tx.saltedHash]; !there {
				if _, there := p.pendingTransactions[tx.saltedHash]; there {
					// tx is now decoded, produce decoded tx
					decodedTx := &Transaction{}
					err := decodedTx.UnmarshalBinary(c.symbol[:])
					if err != nil {
						panic("error unmarshalling degree-1 codeword into transaction")
					}
					p.hasher.Reset()
					p.hasher.Write(decodedTx.hash[:])
					computedHash := (uint32)(p.hasher.Sum64())
					if computedHash != tx.saltedHash {
						failedTx, failed := c.failToDecode()
						if failed {
							delete(p.pendingTransactions, failedTx)
						}
						//TODO: streamline the API
						//panic("hash of decoded transaction does not match codeword header")
					} else {
						newTx = append(newTx, decodedTx)
						delete(p.pendingTransactions, tx.saltedHash)
						p.storeNewTransaction(tx.saltedHash, decodedTx)
						queue = tx.markDecoded(decodedTx, queue)
					}
				} else {
					panic("unpeeled transaction is not pending")
				}
			} else {
				panic("unpeeled transaction is already received")
			}
		} else {
			panic("queued undecodable codeword")
		}
		if len(c.members) != 0 {
			panic("codeword not empty after decoded")
		}
		c.decoded = true
		if c.freed {
			pendingCodewordPool.Put(c)
		}
	}
	return newTx
}

func (p *Decoder) NumTransactionsReceived() int {
	return p.numTransactionsDecoded
}
