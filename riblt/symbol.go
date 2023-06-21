package riblt

type Symbol[T any] interface {
	// XOR returns the XOR result of the method receiver and t2. It is allowed
	// to modify the method receiver during the operation. When the method
	// receiver is the default value of T, the result is t2.
	XOR(t2 T) T
	// Hash returns the cryptographic hash of the method receiver with the
	// given key. It is guaranteed not to modify the method receiver.
	Hash(key uint64) uint64
	comparable
}

type HashedSymbol[T Symbol[T]] struct {
	symbol T
	hash []byte
}

func NewHashedSymbol[T Symbol[T]](s T) HashedSymbol[T] {
	h := HashedSymbol[T]{}
	h.symbol = s 
	h.hash = s.Hash()
	return h
}

func (s HashedSymbol[T]) Symbol() T {
	return s.symbol
}

func (s HashedSymbol[T]) Hash() []byte {
	return s.hash
}
