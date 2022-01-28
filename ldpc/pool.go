package ldpc

import (
//	"github.com/cespare/xxhash"
)

type pendingTransaction struct {
	saltedHash uint64
	blocking []*Codeword
}

type pendingCodeword struct {
	members []*pendingTransaction

}

type PeerState struct {
	saltedTransactionMap map[uint64]*Transaction
	pendingCodewords []*Codeword
}

