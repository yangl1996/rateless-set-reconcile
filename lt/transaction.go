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
	tx := Transaction[T]{}
	// FIXME: this causes an allocation. We could have just take ownership of
	// data passed to us as the argument, but in decodeCodewords in decoder.go
	// we peel newly-decoded transaction from codewords it is blocking,
	// including the one from which the transaction is decoded. If the
	// transaction and the codeword use the same memory space (which will
	// happen if we do not create new tx.data), we will XOR the data with
	// itself and zero it out.  Remove the allocation after we fix this issue.
	tx.data = tx.data.XOR(data)
	tx.hash = tx.data.Hash()
	return tx
}

func (t Transaction[T]) Data() T {
	return t.data
}

func (t Transaction[T]) Hash() []byte {
	return t.hash
}
