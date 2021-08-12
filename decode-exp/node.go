package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"math/rand"
)

type node struct {
	*ldpc.TransactionPool
	dist     thresholdPicker
	rng      *rand.Rand
	pacer    transactionPacer
	lookback uint64
}

func newNode(srcPool []ldpc.Transaction, nCopy, nNew int, dist thresholdPicker, rng *rand.Rand, pacer transactionPacer, lookback uint64) (*node, []ldpc.Transaction) {
	node := &node{}
	node.rng = rng
	node.TransactionPool = &ldpc.TransactionPool{
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
			node.TransactionPool.AddTransaction(tx, ldpc.MaxTimestamp)
			i += 1
			res = append(res, tx)
		}
	}
	for i := 0; i < nNew; i++ {
		tx := node.getRandomTransaction()
		node.TransactionPool.AddTransaction(tx, ldpc.MaxTimestamp)
		res = append(res, tx)
	}
	node.dist = dist
	node.pacer = pacer
	node.lookback = lookback
	return node, res
}

func (n *node) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	n.rng.Read(d[:])
	return ldpc.NewTransaction(d, n.Seq)
}

func (n *node) produceCodeword() ldpc.Codeword {
	return n.TransactionPool.ProduceCodeword(n.rng.Uint64(), n.dist.generate(), n.rng.Intn(ldpc.MaxUintIdx), n.lookback)
}
