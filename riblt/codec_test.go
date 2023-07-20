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

type testDegreeSequence struct {
	count int
}

func (t *testDegreeSequence) Reset() {
	t.count = 0
}

func (t *testDegreeSequence) NextThreshold() uint64 {
	var th uint64
	th = math.MaxUint64
	if t.count != 0 {
		th = uint64(float64(th) / (1 + float64(t.count)/2))
	}
	t.count += 1
	return th
}

func TestSynchronizedEncodeAndDecode(t *testing.T) {
	enc := SynchronizedEncoder[*testSymbol]{rand.New(rand.NewSource(10)), &Encoder[*testSymbol]{}, &testDegreeSequence{}}
	dec := SynchronizedDecoder[*testSymbol]{rand.New(rand.NewSource(10)), &Decoder[*testSymbol]{}, &testDegreeSequence{}}
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
	for len(dec.remote) < nremote || len(dec.local) < nlocal {
		dec.AddNextCodedSymbol(enc.ProduceNextCodedSymbol())
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
		t.Errorf("missing symbols: %d remote and %d local", len(remote), len(local))
	}
	if !dec.Decoded() {
		t.Errorf("decoder not marked as decoded")
	}
	t.Logf("%d codewords until fully decoded", ncw)
}
