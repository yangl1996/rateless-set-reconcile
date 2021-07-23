package ldpc

const NumBucketBits = 8
const NumBuckets = 0x1 << NumBucketBits
const BucketSize uint64 = 0x1 << (64 - NumBucketBits)

type Trie struct {
	Buckets [MaxUintIdx][NumBuckets]TrieBucket
	Counter int
}

type TrieBucket struct {
	Items   []*TimestampedTransaction
	Counter int
}

func (b *TrieBucket) AddTransaction(tx *TimestampedTransaction) {
	b.Counter += 1
	b.Items = append(b.Items, tx)
}

func (t *Trie) AddTransaction(tx *TimestampedTransaction) {
	for i := 0; i < MaxUintIdx; i++ {
		h := tx.Uint(i)
		t.Buckets[i][h/BucketSize].AddTransaction(tx)
	}
	t.Counter += 1
}
