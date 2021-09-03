package ldpc

const numBucketBits = 8
const numBuckets = 0x1 << numBucketBits
const bucketSize uint64 = 0x1 << (64 - numBucketBits)

type trie struct {
	buckets [MaxHashIdx][numBuckets]trieBucket
	counter int
}

type trieBucket struct {
	items   []*timestampedTransaction
	counter int
}

func (b *trieBucket) addTransaction(tx *timestampedTransaction) {
	b.counter += 1
	b.items = append(b.items, tx)
}

func (b *trieBucket) removeItemAt(idx int) {
	newLen := len(b.items) - 1
	b.items[idx].rc -= 1
	if b.items[idx].rc == 0 {
		b.items[idx].hashedTransaction.rc -= 1
		if b.items[idx].hashedTransaction.rc == 0 {
			hashedTransactionPool.Put(b.items[idx].hashedTransaction)
			b.items[idx].hashedTransaction = nil
		}
		timestampPool.Put(b.items[idx])
	}
	b.items[idx] = b.items[newLen]
	b.items[newLen] = nil
	b.items = b.items[0:newLen]
}

func (t *trie) addTransaction(tx *timestampedTransaction) {
	for i := 0; i < MaxHashIdx; i++ {
		h := tx.uint(i)
		t.buckets[i][h/bucketSize].addTransaction(tx)
	}
	t.counter += 1
	tx.rc += MaxHashIdx
}

