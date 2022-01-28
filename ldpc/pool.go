package ldpc

import (
	"github.com/dchest/siphash"
	"hash"
)

type pendingTransaction struct {
	saltedHash uint32
	blocking []*pendingCodeword
}

func (tx *pendingTransaction) markDecoded(preimage *Transaction, decodableCws []*pendingCodeword) []*pendingCodeword {
	// peel all codewords containing this transaction
	for _, peelable := range tx.blocking {
		peelable.peelTransaction(tx, preimage)
		// if it is now decodable, queue it for decoding
		if len(peelable.members) <= 1 && !peelable.queued {
			peelable.queued = true
			decodableCws = append(decodableCws, peelable)
		}
	}
	// we do not need to set each item in tx.blocking to nil
	// because the whole slice (and the backing array) is going out
	// of scope with tx
	return decodableCws
}

type pendingCodeword struct {
	symbol TransactionData
	members []*pendingTransaction
	queued bool
}

func (peelable *pendingCodeword) peelTransaction(stub *pendingTransaction, preimage *Transaction) {
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

type peerState struct {
	receivedTransactions  map[uint32]*Transaction
	pendingTransactions map[uint32]*pendingTransaction
	hasher hash.Hash64
}

func newPeer(salt []byte) *peerState {
	p := &peerState{
		receivedTransactions: make(map[uint32]*Transaction),
		pendingTransactions: make(map[uint32]*pendingTransaction),
		hasher: siphash.New(salt),
	}
	return p
}

func (p *peerState) addTransaction(t *Transaction) []*Transaction {
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
func (p *peerState) decodeCodewords(queue []*pendingCodeword) []*Transaction {
	newTx := []*Transaction{}

	for _, c := range queue {
		// we do not want to add the same codeword to the queue twice, so we mark
		// it as queued
		c.queued = true
	}
	for len(queue) > 0 {
		// pop the last item from the queue (stack)
		c := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
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
					newTx = append(newTx, decodedTx);
					delete(p.pendingTransactions, tx.saltedHash)
					p.receivedTransactions[tx.saltedHash] = decodedTx
					queue = tx.markDecoded(decodedTx, queue)
				} else {
					panic("unpeeled transaction is not pending")
				}
			} else {
				panic("unpeeled transaction is already received")
			}
		} else {
			panic("queued undecodable codeword")
		}
	}
	return newTx
}
