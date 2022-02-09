package ldpc

type Codeword struct {
	Symbol  TransactionData // XOR of all transactions put into this codeword
	Members []uint32        // salted hashes of transactions
}
