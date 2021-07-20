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

func newNode(srcPool *ldpc.TransactionPool, nCopy, nNew int, dist thresholdPicker, rng *rand.Rand, pacer transactionPacer) (*node, error) {
	node := &node{}
	node.rng = rng
	var err error
	node.TransactionPool, err = ldpc.NewTransactionPool()
	if err != nil {
		return node, err
	}

	if srcPool != nil {
		i := 0
		for tx, _ := range srcPool.TransactionId {
			if i >= nCopy {
				break
			}
			node.TransactionPool.AddTransaction(tx)
			i += 1
		}
	}
	for i := 0; i < nNew; i++ {
		node.TransactionPool.AddTransaction(node.getRandomTransaction())
	}
	node.dist = dist
	node.pacer = pacer
	return node, nil
}

func (n *node) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	n.rng.Read(d[:])
	return ldpc.NewTransaction(d, uint64(n.Seq))
}

func (n *node) produceCodeword() ldpc.Codeword {
	return n.TransactionPool.ProduceCodeword(n.rng.Uint64(), n.dist.generate(), n.rng.Intn(ldpc.MaxUintIdx))
}
