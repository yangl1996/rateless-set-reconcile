package riblt

import (
	"encoding/binary"
	"math/rand"
	"math"
	"testing"
	"github.com/dchest/siphash"
)

const simpleDataSize = 256

type simpleData [simpleDataSize]byte

func (d *simpleData) XOR(t2 *simpleData) *simpleData {
	if d == nil {
		d = &simpleData{}
	}
	for i := 0; i < simpleDataSize; i++ {
		d[i] ^= t2[i]
	}
	return d
}

func (d *simpleData) Hash() uint64 {
	return siphash.Hash(567, 890, d[:])
}

func newSimpleData(i uint64) *simpleData {
	data := simpleData{}
	binary.LittleEndian.PutUint64(data[0:8], i)
	return &data
}

func TestEncodeAndDecode(t *testing.T) {
	ndiff := 50000
	e := Encoder[*simpleData]{}
	for i := 0; i < ndiff; i++ {
		s := NewHashedSymbol[*simpleData](newSimpleData(uint64(i)))
		e.AddSymbol(s)
	}
	dec := Decoder[*simpleData]{}
	ncw := 0
	for len(dec.added) < ndiff {
		salt0 := rand.Uint64()
		salt1 := rand.Uint64()
		var th uint64
		th = math.MaxUint64
		if ncw != 0 {
			th = uint64(float64(th) / (1 + float64(ncw)/2))
		}
		c := e.ProduceCodedSymbol(salt0, salt1, th)
		dec.AddCodedSymbol(c, salt0, salt1, th)
		ncw += 1
	}
	t.Logf("%d codewords until fully decoded", ncw)
}
