package main

import (
	"time"
	"encoding/binary"
	"github.com/yangl1996/rateless-set-reconcile/lt"
)

type transaction uint64

func (d transaction) XOR(t2 transaction) transaction {
	return d ^ t2
}

func (d transaction) Hash() []byte {
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, uint64(d))
	return res
}

func (d transaction) Equals(t2 transaction) bool {
	return d == t2
}

type transactionGenerator struct {
	next uint64
	ts map[transaction]time.Duration
}

func newTransactionGenerator() *transactionGenerator {
	return &transactionGenerator{
		ts: make(map[transaction]time.Duration),
	}
}

func (t *transactionGenerator) generate(at time.Duration) lt.Transaction[transaction] {
	tx := transaction(t.next)
	t.ts[tx] = at
	t.next += 1
	return lt.NewTransaction[transaction](tx)
}

func (t *transactionGenerator) timestamp(tx transaction) time.Duration {
	return t.ts[tx]
}
