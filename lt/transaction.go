package lt

type TransactionData interface {
	XOR(t2 TransactionData)
	Equals(t2 TransactionData) bool
	Hash() []byte
}

type Transaction[T TransactionData] struct {
	data T
	hash []byte
}

func NewTransaction[T TransactionData](data T) Transaction[T] {
	return Transaction[T]{data, data.Hash()}
}

func (t Transaction[T]) Data() T {
	return t.data
}

func (t Transaction[T]) Hash() []byte {
	return t.hash
}
