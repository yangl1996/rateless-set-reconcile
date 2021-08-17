package main

import (
	"math/rand"

	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type node struct {
	*ldpc.TransactionSync
	dist             thresholdPicker
	rng              *rand.Rand
	transactionPacer pacer
	lookback         uint64
	peers []struct{*node; int}
	decoded int
}

func newNode(dist thresholdPicker, rng *rand.Rand, txPacer pacer, lookback uint64) *node {
	node := &node{}
	node.rng = rng
	node.TransactionSync = &ldpc.TransactionSync{
		SyncClock: ldpc.SyncClock{
			TransactionTimeout: lookback,
			CodewordTimeout:    lookback * 5,
			Seq:                1,
		},
	}
	node.dist = dist
	node.transactionPacer = txPacer
	node.lookback = lookback
	return node
}

func (n *node) connectTo(peer *node) {
	n.TransactionSync.AddPeer()
	peer.TransactionSync.AddPeer()
	n.peers = append(n.peers, struct{*node; int}{
		peer,
		len(peer.TransactionSync.PeerStates)-1,
	})
	peer.peers = append(peer.peers, struct{*node; int}{
		n,
		len(n.TransactionSync.PeerStates)-1,
	})
}

func (n *node) fillInitTransaction(src []ldpc.Transaction, nCopy, nNew int) []ldpc.Transaction {
	var res []ldpc.Transaction
	if src != nil {
		i := 0
		for _, tx := range src {
			if i >= nCopy {
				break
			}
			n.TransactionSync.AddLocalTransaction(tx)
			i += 1
			res = append(res, tx)
		}
	}

	for i := 0; i < nNew; i++ {
		tx := n.getRandomTransaction()
		n.TransactionSync.AddLocalTransaction(tx)
		res = append(res, tx)
	}
	return res
}

func (n *node) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	n.rng.Read(d[:])
	return ldpc.NewTransaction(d, n.Seq)
}

func (n *node) sendCodewords() {
	np := len(n.peers)
	for pidx := 0; pidx < np; pidx++ {
		cw := n.TransactionSync.PeerStates[pidx].ProduceCodeword(
			n.rng.Uint64(),
			n.dist.generate(),
			n.rng.Intn(ldpc.MaxHashIdx),
			n.lookback,
		)
		ourIdx := n.peers[pidx].int
		n.peers[pidx].node.PeerStates[ourIdx].InputCodeword(cw)
	}
	n.Seq += 1
}

func (n *node) tryDecode() int {
	lastNum := n.txPoolSize()
	n.TryDecode()
	newNum := n.txPoolSize()
	n.decoded += (newNum - lastNum)
	return newNum - lastNum
}

func (n *node) txPoolSize() int {
	return n.TransactionSync.PeerStates[0].NumAddedTransactions()
}
