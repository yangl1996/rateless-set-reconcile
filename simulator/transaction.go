package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
    "math/rand"
	"time"
	"hash/maphash"
)

type transactionGenerator struct {
	ts map[uint64]time.Duration
	seed maphash.Seed
}

func newTransactionGenerator() *transactionGenerator {
	return &transactionGenerator{
		ts: make(map[uint64]time.Duration),
		seed: maphash.MakeSeed(),
	}
}

func (t *transactionGenerator) generate(at time.Duration) *ldpc.Transaction {
	d := ldpc.TransactionData{}
	rand.Read(d[:])
	h := maphash.Bytes(t.seed, d[:])
	t.ts[h] = at
	tx := &ldpc.Transaction{}
	tx.UnmarshalBinary(d[:])
	return tx
}

func (t *transactionGenerator) timestamp(tx *ldpc.Transaction) time.Duration {
	h := maphash.Bytes(t.seed, tx.Serialized())
	return t.ts[h]
}
