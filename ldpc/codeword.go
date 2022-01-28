package ldpc

const hashLength = 4

type Codeword struct {
	symbol  TransactionData	// XOR of all transactions put into this codeword
	members []uint64	// hashes of transactions; only the lowest hashLength bytes are valid
}

