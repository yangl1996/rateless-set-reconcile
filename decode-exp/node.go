package main

import (
	"math/rand"

	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type node struct {
	*ldpc.PeerSyncState
	dist             thresholdPicker
	rng              *rand.Rand
	transactionPacer pacer
	lookback         uint64
}

func newNode(srcPool []ldpc.Transaction, nCopy, nNew int, dist thresholdPicker, rng *rand.Rand, txPacer pacer, lookback uint64) (*node, []ldpc.Transaction) {
	node := &node{}
	node.rng = rng
	node.PeerSyncState = &ldpc.PeerSyncState{
		TransactionTimeout: lookback,
		CodewordTimeout:    lookback * 5,
		Seq:                1,
	}
	res := make([]ldpc.Transaction, 0, nCopy+nNew)

	if srcPool != nil {
		i := 0
		for _, tx := range srcPool {
			if i >= nCopy {
				break
			}
			node.PeerSyncState.AddTransaction(tx, ldpc.MaxTimestamp)
			i += 1
			res = append(res, tx)
		}
	}
	for i := 0; i < nNew; i++ {
		tx := node.getRandomTransaction()
		node.PeerSyncState.AddTransaction(tx, ldpc.MaxTimestamp)
		res = append(res, tx)
	}
	node.dist = dist
	node.transactionPacer = txPacer
	node.lookback = lookback
	return node, res
}

func (n *node) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	n.rng.Read(d[:])
	return ldpc.NewTransaction(d, n.Seq)
}

func (n *node) produceCodeword() ldpc.Codeword {
	return n.PeerSyncState.ProduceCodeword(n.rng.Uint64(), n.dist.generate(), n.rng.Intn(ldpc.MaxHashIdx), n.lookback)
}
