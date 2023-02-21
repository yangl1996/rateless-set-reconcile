package lt

type TransactionData[T any] interface {
	XOR(t2 T) T	// XOR is allowed to modify the method receiver
	Equals(t2 T) bool
	Hash() []byte
}

type Transaction[T TransactionData[T]] struct {
	data T
	hash []byte
}

func NewTransaction[T TransactionData[T]](data T) Transaction[T] {
	return Transaction[T]{data, data.Hash()}
}

func (t Transaction[T]) Data() T {
	return t.data
}

func (t Transaction[T]) Hash() []byte {
	return t.hash
}
