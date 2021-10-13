package main

import (
	"math/rand"

	"github.com/yangl1996/rateless-set-reconcile/ldpc"
)

type txsource struct {
	pacer
	targets []*node
	rng *rand.Rand
}

func newSource(cfg Source, rng *rand.Rand, nodes map[string]*node) (*txsource, error) {
	pc, err := NewTransactionPacer(rng, cfg.ArrivePattern)
	if err != nil {
		return nil, err
	}
	var targets []*node
	for _, nodeName := range cfg.Targets {
		targets = append(targets, nodes[nodeName])
	}
	src := &txsource{pc, targets, rng}
	for i := 0; i < cfg.InitialTx; i++ {
		tx := src.getRandomTransaction()
		for _, n := range targets {
			n.TransactionSync.AddLocalTransaction(tx)
		}
	}
	return src, nil
}

func (t *txsource) getRandomTransaction() ldpc.Transaction {
	d := [ldpc.TxDataSize]byte{}
	t.rng.Read(d[:])
	return ldpc.NewTransaction(d)
}

func (t *txsource) generate() int {
	nadd := t.pacer.tick()
	for cnt := 0; cnt < nadd; cnt++ {
		tx := t.getRandomTransaction()
		for _, target := range t.targets {
			target.AddLocalTransaction(tx)
		}
	}
	return nadd
}

type node struct {
	*ldpc.TransactionSync
	dist             thresholdPicker
	rng              *rand.Rand
	lookback         uint64
	peers []struct{*node; int}
	decoded int
	cwrcvd int
	lastAct uint64
	declimit int
	timeout int
	name string
}

func newNode(cfg Server, rng *rand.Rand) (*node, error) {
	// calculate lookback window, mind overflows
	txLookback := cfg.LookbackTime * 5
	if txLookback < cfg.LookbackTime {
		txLookback = ldpc.MaxTimestamp
	}
	cwLookback := cfg.LookbackTime * 5
	if cwLookback < cfg.LookbackTime {
		cwLookback = ldpc.MaxTimestamp
	}
	dist, err := NewDistribution(rng, cfg.DegreeDist)
	if err != nil {
		return nil, err
	}
	node := &node{
		TransactionSync: &ldpc.TransactionSync{
			SyncClock: ldpc.SyncClock{
				TransactionTimeout: txLookback,
				CodewordTimeout:    cwLookback,
				Seq:                1,
			},
		},
		dist: dist,
		rng: rng,
		lookback: cfg.LookbackTime,
		declimit: cfg.TimeoutCounter,
		timeout: cfg.TimeoutDuration,
		name: cfg.Name,
	}
	return node, nil
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
		n.peers[pidx].node.cwrcvd += 1
	}
	n.Seq += 1
}

func (n *node) tryDecode() int {
	lastNum := n.txPoolSize()
	n.TryDecode()
	newNum := n.txPoolSize()
	n.decoded += (newNum - lastNum)
	if newNum != lastNum {
		n.lastAct = n.Seq
	}
	return newNum - lastNum
}

func (n *node) txPoolSize() int {
	return n.TransactionSync.PeerStates[0].NumAddedTransactions()
}
