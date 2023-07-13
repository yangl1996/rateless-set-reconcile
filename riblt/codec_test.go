package riblt

import (
	"encoding/binary"
	"math/rand"
	"math"
	"testing"
	"github.com/dchest/siphash"
)

const testSymbolSize = 256

type testSymbol [testSymbolSize]byte

func (d *testSymbol) XOR(t2 *testSymbol) *testSymbol {
	if d == nil {
		d = &testSymbol{}
	}
	for i := 0; i < testSymbolSize; i++ {
		d[i] ^= t2[i]
	}
	return d
}

func (d *testSymbol) Hash() uint64 {
	return siphash.Hash(567, 890, d[:])
}

func newTestSymbol(i uint64) *testSymbol {
	data := testSymbol{}
	binary.LittleEndian.PutUint64(data[0:8], i)
	return &data
}

func TestEncodeAndDecode(t *testing.T) {
	enc := Encoder[*testSymbol]{}
	dec := Decoder[*testSymbol]{}
	local := make(map[uint64]struct{})
	remote := make(map[uint64]struct{})

	var nextId uint64
	nlocal := 5000
	nremote := 5000
	ncommon := 10000
	for i := 0; i < ncommon; i++ {
		s := newTestSymbol(nextId)
		nextId += 1
		enc.AddSymbol(s)
		dec.AddSymbol(s)
	}
	for i := 0; i < nlocal; i++ {
		s := newTestSymbol(nextId)
		nextId += 1
		dec.AddSymbol(s)
		local[s.Hash()] = struct{}{}
	}
	for i := 0; i < nremote; i++ {
		s := newTestSymbol(nextId)
		nextId += 1
		enc.AddSymbol(s)
		remote[s.Hash()] = struct{}{}
	}

	ncw := 0
	for len(dec.remote) < nremote && len(dec.local) < nlocal {
		salt := rand.Uint64()
		var th uint64
		th = math.MaxUint64
		if ncw != 0 {
			th = uint64(float64(th) / (1 + float64(ncw)/2))
		}
		cw := enc.ProduceCodedSymbol(salt, th)
		dec.AddCodedSymbol(cw, salt, th)
		ncw += 1
		dec.TryDecode()
	}
	for _, v := range dec.remote {
		delete(remote, v.Hash)
	}
	for _, v := range dec.local {
		delete(local, v.Hash)
	}
	if len(remote) != 0 || len(local) != 0 {
		t.Errorf("missing symbols")
	}
	t.Logf("%d codewords until fully decoded", ncw)
}
