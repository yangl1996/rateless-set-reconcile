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
	set := make(map[uint64]struct{})
	ndiff := 10000
	e := Encoder[*simpleData]{}
	for i := 0; i < ndiff; i++ {
		d := newSimpleData(uint64(i))
		e.AddSymbol(d)
		set[d.Hash()] = struct{}{}
	}
	dec := Decoder[*simpleData]{}
	ncw := 0
	for len(dec.added) < ndiff {
		salt := rand.Uint64()
		var th uint64
		th = math.MaxUint64
		if ncw != 0 {
			th = uint64(float64(th) / (1 + float64(ncw)/2))
		}
		c := e.ProduceCodedSymbol(salt, th)
		dec.AddCodedSymbol(c, salt, th)
		ncw += 1
	}
	for _, v := range dec.added {
		delete(set, v.Hash)
	}
	if len(set) != 0 {
		t.Errorf("missing symbols")
	}
	t.Logf("%d codewords until fully decoded", ncw)
}
