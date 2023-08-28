package riblt

// TODO: encoder should send conflicting transactions as-is when detected; since it happens very rarely and each pair of peers can use a secret hash key, an adversary cannot forge too many conflicts.
// TODO: replace siphash with xxhash (or whatever that supports native 4-byte output)

type CodedSymbol[T Symbol[T]] struct {
	sum T
	count int64
	checksum uint64
}

type inWindowSymbol[T Symbol[T]] struct {
	HashedSymbol[T]
	randomMapping
}

type symbolMapping struct {
	sourceIdx int
	codedIdx int
}

// TODO: remove the heap?
type mappingHeap []symbolMapping

func (m mappingHeap) fixHead() {
	curr := 0
	for {
		child := curr * 2 + 1
		if child >= len(m) {
			// no left child
			break
		}
		if rc := child + 1; rc < len(m) && m[rc].codedIdx < m[child].codedIdx {
			child = rc
		}
		if m[curr].codedIdx <= m[child].codedIdx {
			break
		}
		m[curr], m[child] = m[child], m[curr]
		curr = child
	}
}

func (m mappingHeap) fixTail() {
	curr := len(m)-1
	for {
		parent := (curr - 1) / 2
		if curr == parent || m[parent].codedIdx <= m[curr].codedIdx {
			break
		}
		m[parent], m[curr] = m[curr], m[parent]
		curr = parent
	}
}

type Encoder[T Symbol[T]] struct {
	window []inWindowSymbol[T]
	mapping mappingHeap
	nextIdx int
}

func (e *Encoder[T]) AddHashedSymbol(t HashedSymbol[T]) {
	e.window = append(e.window, inWindowSymbol[T]{t, randomMapping{t.Hash, 0}})
	e.mapping = append(e.mapping, symbolMapping{len(e.window)-1, 0})
	e.mapping.fixTail()
}

func (e *Encoder[T]) AddSymbol(t T) {
	th := HashedSymbol[T]{t, t.Hash()}
	e.AddHashedSymbol(th)
}

func (e *Encoder[T]) ProduceNextCodedSymbol() CodedSymbol[T] {
	c := CodedSymbol[T]{}
	for e.mapping[0].codedIdx <= e.nextIdx {
		v := e.window[e.mapping[0].sourceIdx]
		c.sum = c.sum.XOR(v.Symbol)
		c.count += 1
		c.checksum ^= v.Hash
		// generate the next mapping
		nextMap := e.window[e.mapping[0].sourceIdx].nextIndex()
		e.mapping[0].codedIdx = int(nextMap)
		e.mapping.fixHead()
	}
	e.nextIdx += 1

	return c
}

func (e *Encoder[T]) Reset() {
	if len(e.window) != 0 {
		e.window = e.window[:0]
	}
	if len(e.mapping) != 0 {
		e.mapping = e.mapping[:0]
	}
	e.nextIdx = 0
}

