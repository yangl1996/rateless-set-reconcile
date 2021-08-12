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

func (t *trie) addTransaction(tx *timestampedTransaction) {
	for i := 0; i < MaxHashIdx; i++ {
		h := tx.uint(i)
		t.buckets[i][h/bucketSize].addTransaction(tx)
	}
	t.counter += 1
}
