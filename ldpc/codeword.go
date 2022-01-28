package ldpc

type Codeword struct {
	symbol  TransactionData	// XOR of all transactions put into this codeword
	members []uint32	    // salted hashes of transactions
}

