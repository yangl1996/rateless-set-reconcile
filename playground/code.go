package playground

import (
	"golang.org/x/crypto/blake2b"
)

const TxSize = 512

// ProduceCodeword hashes each element with blake2b-256 and XOR those
// whose hash fall below a certain threshold. Specifically, we pick
// hashes whose first byte (0-255) is smaller than frac.
func ProduceCodeword(d [][TxSize]byte, frac int) ([TxSize]byte, error) {
	res := [TxSize]byte{}
	// get a hasher. we do not want to allocate over and over again
	hasher, err := blake2b.New256(nil)
	if err != nil {
		return res, err
	}
	for _, v := range d {
		hasher.Reset()
		hasher.Write(v[:])
		if int(hasher.Sum(nil)[0]) < frac {
			for i := 0; i < TxSize; i++ {
				//res[i] = res[i] ^ v[i]
			}
		}
	}
	return res, nil
}
