package riblt

type Sketch[T Symbol[T]] []CodedSymbol[T]

func (s Sketch[T]) AddHashedSymbol(t HashedSymbol[T]) {
	m := randomMapping{t.Hash, 0}
	for int(m.lastIdx) < len(s) {
		s[m.lastIdx] = s[m.lastIdx].apply(t, add)
		m.nextIndex()
	}
}

func (s Sketch[T]) AddSymbol(t T) {
	hs := HashedSymbol[T]{t, t.Hash()}
	m := randomMapping{hs.Hash, 0}
	for int(m.lastIdx) < len(s) {
		s[m.lastIdx] = s[m.lastIdx].apply(hs, add)
		m.nextIndex()
	}
}

func (s Sketch[T]) Subtract(s2 Sketch[T]) Sketch[T] {
	if len(s) != len(s2) {
		panic("subtracting sketches of different sizes")
	}

	for i := range s {
		s[i].sum = s[i].sum.XOR(s2[i].sum)
		s[i].count = s[i].count - s2[i].count
		s[i].checksum = s[i].checksum ^ s2[i].checksum
	}
	return s
}

