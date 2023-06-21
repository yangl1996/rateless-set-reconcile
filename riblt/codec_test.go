package riblt

import (
	"encoding/binary"
	"math/rand"
	"math"
	"testing"
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

func (d *simpleData) Hash() []byte {
	return d[:]
}

func newSimpleData(i uint64) *simpleData {
	data := simpleData{}
	binary.LittleEndian.PutUint64(data[0:8], i)
	return &data
}

func TestEncodeAndDecode(t *testing.T) {
	e := Encoder[*simpleData]{}
	for i := 0; i < 5000; i++ {
		s := NewHashedSymbol[*simpleData](newSimpleData(uint64(i)))
		e.AddSymbol(s)
	}
	dec := Decoder[*simpleData]{}
	ncw := 0
	for len(dec.added) < 5000 {
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
