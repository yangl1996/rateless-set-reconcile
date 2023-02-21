package lt

import (
	"github.com/dchest/siphash"
	"hash"
	//"sync"
)

/*
var pendingTransactionPool = sync.Pool{
	New: func() interface{} {
		return &pendingTransaction{}
	},
}
*/

type pendingTransaction[T TransactionData] struct {
	saltedHash uint32
	blocking   []*PendingCodeword[T]
}

func (tx *pendingTransaction[T]) reset() {
	tx.blocking = tx.blocking[:0]
}

// markDecoded marks tx as decoded, scans the list of codewords blocked by tx
// and peels data from them, and appends newly-decodable codewords into
// decodableCws. tx will never be used after this operation.
func (tx *pendingTransaction[T]) markDecoded(data T, decodableCws []*PendingCodeword[T]) []*PendingCodeword[T] {
	// peel all codewords containing this transaction
	for idx, peelable := range tx.blocking {
		peelable.peelTransaction(tx, data)
		// if peelable is now decodable, queue it for decoding
		if len(peelable.members) <= 1 && !peelable.queued {
			peelable.queued = true
			decodableCws = append(decodableCws, peelable)
		}
		// set the pointer to nil to free peelable
		tx.blocking[idx] = nil
	}
	return decodableCws
}

type PendingCodeword[T TransactionData] struct {
	symbol  T 
	members []*pendingTransaction[T]
	queued  bool
	decoded bool
}

func (cw *PendingCodeword[T]) Decoded() bool {
	return cw.decoded
}

// failToDecode marks that cw cannot be decoded, probably because of hash conflicts.
// It returns the hash of and the pointer to the blocking pending transaction along with 
// true when the blocking pending transaction cannot be decoded and can be freed, and
// false if otherwise.
// TODO: can we record the history of decoding (which transactions get peeled), and
// recover by re-applying these transactions and trying different transactions with
// the same hash?
func (cw *PendingCodeword[T]) failToDecode() (uint32, *pendingTransaction[T], bool) {
	if len(cw.members) != 1 {
		panic("failing a codeword when it has more than 1 members")
	}
	ptr := cw.members[0]
	// remove cw from the blocking list of all pending transactions
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
			// see if the pending transaction is blocking nothing else
			if len(ptr.blocking) == 0 {
				return ptr.saltedHash, ptr, true
			} else {
				return 0, nil, false
			}
		}
	}
	panic("unable to find blocked codeword in pending transaction")
}

func (peelable *PendingCodeword[T]) peelTransaction(stub *pendingTransaction[T], data T) {
	for idx, ptr := range peelable.members {
		if ptr == stub {
			l := len(peelable.members)
			// pop the ptr by swapping with the last item
			// we need to set the deleted ptr to nil to
			// free the pointed value
			peelable.members[idx] = peelable.members[l-1]
			peelable.members[l-1] = nil
			peelable.members = peelable.members[:l-1]
			peelable.symbol.XOR(data)
			return
		}
	}
	panic("unable to peel decoded transaction from codeword pointing to it")
}

type Decoder[T TransactionData] struct {
	receivedTransactions map[uint32]Transaction[T]
	recentTransactions []saltedTransaction[T]
	pendingTransactions  map[uint32]*pendingTransaction[T]
	hasher               hash.Hash64
	memory int
}

func NewDecoder[T TransactionData](salt [SaltSize]byte, memory int) *Decoder[T] {
	p := &Decoder[T]{
		receivedTransactions: make(map[uint32]Transaction[T]),
		pendingTransactions:  make(map[uint32]*pendingTransaction[T]),
		hasher:               siphash.New(salt[:]),
		memory: memory,
	}
	return p
}

func (p *Decoder[T]) storeNewTransaction(saltedHash uint32, t Transaction[T]) {
	p.receivedTransactions[saltedHash] = t
	p.recentTransactions = append(p.recentTransactions, saltedTransaction[T]{saltedHash, t})
	// free receiver memory if needed
	for len(p.recentTransactions) > p.memory {
		// FIXME: the comparison of the hashes is a hack. It is a shallow comparison, but currently it is fine because
		// Transaction values in receivedTransactions and recentTransactions have the same origin. The shallow comparison
		// has the same effect as a pointer comparison.
		if e := p.receivedTransactions[p.recentTransactions[0].saltedHash]; &e.hash[0] == &p.recentTransactions[0].Transaction.hash[0] {
			delete(p.receivedTransactions, p.recentTransactions[0].saltedHash)
		}
		p.recentTransactions = p.recentTransactions[1:]
	}
}

func (p *Decoder[T]) AddCodeword(rawCodeword Codeword[T]) (*PendingCodeword[T], []Transaction[T]) {
	cw := &PendingCodeword[T]{}
	cw.symbol = rawCodeword.symbol
	for _, member := range rawCodeword.members {
		pending, pendingExists := p.pendingTransactions[member]
		received, receivedExists := p.receivedTransactions[member]
		if !receivedExists {
			if !pendingExists {
				// we have never heard of it
				pending = &pendingTransaction[T]{}
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
				cw.symbol.XOR(received.data)
			} else {
				panic("transaction is marked both received and pending")
			}
		}
	}
	if len(cw.members) <= 1 {
		cw.queued = true
		queue := []*PendingCodeword[T]{cw}
		txs := p.decodeCodewords(queue)
		return cw, txs
	}
	return cw, nil
}

func (p *Decoder[T]) AddTransaction(t Transaction[T]) []Transaction[T] {
	p.hasher.Reset()
	p.hasher.Write(t.hash[:])
	saltedHash := (uint32)(p.hasher.Sum64())
	if existing, there := p.receivedTransactions[saltedHash]; !there {
		p.storeNewTransaction(saltedHash, t)
		if pending, there := p.pendingTransactions[saltedHash]; there {
			// quick sanity check
			if pending.saltedHash != saltedHash {
				panic("salted hash of retrieved transaction stub does not match the hash computed from the full transaction")
			}
			// peel the transaction and try decoding
			delete(p.pendingTransactions, saltedHash)
			queue := pending.markDecoded(t.data, nil)
			return p.decodeCodewords(queue)
		} else {
			return nil
		}
	} else {
		if existing.data.Equals(t.data) {
			// something that we already know; do not do anything
			return nil
		} else {
			// adding a transaction that is a hash conflict with an existing one that we have not forgotten
			p.storeNewTransaction(saltedHash, t)
			if _, there := p.pendingTransactions[saltedHash]; there {
				panic("pending transaction is already decoded")
			} else {
				return nil
			}
		}
	}
}

// decodeCodewords decodes the list of codewords cws, and returns the list of
// transactions decoded. It updates its local receivedTransactions set.
func (p *Decoder[T]) decodeCodewords(queue []*PendingCodeword[T]) []Transaction[T] {
	newTx := []Transaction[T]{}
	for len(queue) > 0 {
		// pop the last item from the queue
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
			decodableTx := c.members[0]
			// TODO: the following two checks are just for sanity and are potentially
			// costly
			if _, there := p.receivedTransactions[decodableTx.saltedHash]; !there {
				if _, there := p.pendingTransactions[decodableTx.saltedHash]; there {
					// tx is now decoded, produce decoded tx
					decodedTx := NewTransaction[T](c.symbol)
					p.hasher.Reset()
					p.hasher.Write(decodedTx.hash)
					saltedHash := (uint32)(p.hasher.Sum64())
					if saltedHash != decodableTx.saltedHash {
						failedTxSaltedHash, _, failed := c.failToDecode()
						if failed {
							delete(p.pendingTransactions, failedTxSaltedHash)
						}
						//TODO: streamline the API
						//panic("hash of decoded transaction does not match codeword header")
					} else {
						newTx = append(newTx, decodedTx) // c.freed means that the user has timed out the tx
						delete(p.pendingTransactions, saltedHash)
						p.storeNewTransaction(saltedHash, decodedTx)
						queue = decodableTx.markDecoded(decodedTx.data, queue)
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
	}
	return newTx
}

