package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"math/rand"
)

type node struct {
	*ldpc.TransactionPool
	dist  thresholdPicker
	rng   *rand.Rand
	pacer transactionPacer
}

func newNode(srcPool []ldpc.Transaction, nCopy, nNew int, dist thresholdPicker, rng *rand.Rand, pacer transactionPacer) (*node, []ldpc.Transaction, error) {
	node := &node{}
	node.rng = rng
	var err error
	node.TransactionPool, err = ldpc.NewTransactionPool()
	if err != nil {
		return node, nil, err
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
	return node, res, nil
}

func (n *node) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	n.rng.Read(d[:])
	return ldpc.NewTransaction(d, uint64(n.Seq))
}

func (n *node) produceCodeword() ldpc.Codeword {
	return n.TransactionPool.ProduceCodeword(n.rng.Uint64(), n.dist.generate(), n.rng.Intn(ldpc.MaxUintIdx))
}
