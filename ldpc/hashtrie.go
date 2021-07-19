package ldpc

const NumBucketBits = 16
const NumBuckets = 0x1 << NumBucketBits
const BucketSize uint64 = 0x1 << (64 - NumBucketBits)

type Trie struct {
	Buckets [MaxUintIdx][NumBuckets]TrieBucket
	Counter int
}

type TrieBucket struct {
	Items []*TimestampedTransaction
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

func (t *Trie) BucketsInRange(idx int, r HashRange) ([]TrieBucket, []TrieBucket) {
	start := r.start / BucketSize
	end := r.end / BucketSize
	if r.cyclic {
		if end == start {
			return t.Buckets[idx][:], nil
		} else if end < start {
			return t.Buckets[idx][start:NumBuckets], t.Buckets[idx][0:end+1]
		} else {
			panic("corrupted range")
		}
	} else {
		return t.Buckets[idx][start:end+1], nil
	}
}
