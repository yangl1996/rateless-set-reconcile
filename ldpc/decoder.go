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

type Decoder struct {
	receivedTransactions map[uint32]*Transaction
	pendingTransactions  map[uint32]*pendingTransaction
	hasher               hash.Hash64
}

func NewDecoder(salt [SaltSize]byte) *Decoder {
	p := &Decoder{
		receivedTransactions: make(map[uint32]*Transaction),
		pendingTransactions:  make(map[uint32]*pendingTransaction),
		hasher:               siphash.New(salt[:]),
	}
	return p
}

func (p *Decoder) AddCodeword(rawCodeword *Codeword) (*PendingCodeword, []*Transaction) {
	cw := pendingCodewordPool.Get().(*PendingCodeword)
	cw.reset()
	cw.symbol = rawCodeword.symbol
	for _, member := range rawCodeword.members {
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
	if _, there := p.receivedTransactions[hash]; !there {
		p.receivedTransactions[hash] = t
		if pending, there := p.pendingTransactions[hash]; there {
			// quick sanity check
			if pending.saltedHash != hash {
				panic("salted hash of retrieved transaction stub does not match the hash computed from the full transaction")
			}
			// peel the transaction and try decoding
			delete(p.pendingTransactions, hash)
			queue := pending.markDecoded(t, nil)
			// we can free t now
			pendingTransactionPool.Put(pending)
			return p.decodeCodewords(queue)
		} else {
			return nil
		}
	} else {
		panic("adding transaction already decoded")
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
					newTx = append(newTx, decodedTx)
					delete(p.pendingTransactions, tx.saltedHash)
					p.receivedTransactions[tx.saltedHash] = decodedTx
					queue = tx.markDecoded(decodedTx, queue)
					pendingTransactionPool.Put(tx)
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
	return len(p.receivedTransactions)
}
