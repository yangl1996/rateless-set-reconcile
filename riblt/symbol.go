package riblt

type Symbol[T any] interface {
	// XOR returns the XOR result of the method receiver and t2. It is allowed
	// to modify the method receiver during the operation. When the method
	// receiver is the default value of T, the result is equal to t2.
	XOR(t2 T) T
	// Hash returns the cryptographic hash of the method receiver. It is guaranteed not to modify the method receiver.
	Hash() uint64
}

type HashedSymbol[T Symbol[T]] struct {
	Symbol T
	Hash uint64
}

