package main

import (
	"math/rand"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type node struct {
	*ldpc.TransactionPool
	dist thresholdPicker
	rng *rand.Rand
}

func newNode(srcPool *ldpc.TransactionPool, nCopy, nNew int, dist thresholdPicker, rng *rand.Rand) (*node, error) {
	node := &node{}
	node.rng = rng
	var err error
	node.TransactionPool, err = ldpc.NewTransactionPool()
	if err != nil {
		return node, err
	}

	if srcPool != nil {
		i := 0
		for tx, _ := range srcPool.Transactions {
			if i >= nCopy {
				break
			}
			node.TransactionPool.AddTransaction(tx.Transaction)
			i += 1
		}
	}
	for i := 0; i < nNew; i++ {
		node.TransactionPool.AddTransaction(node.getRandomTransaction())
	}
	node.dist = dist
	return node, nil
}

func (n *node) getRandomTransaction() ldpc.Transaction {
        d := [ldpc.TxDataSize]byte{}
        n.rng.Read(d[:])
        return ldpc.NewTransaction(d)
}

func (n *node) produceCodeword() ldpc.Codeword {
	return n.TransactionPool.ProduceCodeword(n.rng.Uint64(), n.dist.generate(), n.rng.Intn(ldpc.MaxUintIdx))
}

