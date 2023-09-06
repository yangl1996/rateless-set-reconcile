package main

import (
	"encoding/binary"
	"github.com/dchest/siphash"
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"time"
	"fmt"
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

func testSymbols(n int) []testSymbol {
	fmt.Println("allocating memory")
	data := make([]testSymbol, n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint64(data[i][0:8], uint64(i))
		if i % 10000 == 0 {
			fmt.Println(i, "symbols created")
		}
	}
	return data
}

func main() {
	enc := riblt.Encoder[*testSymbol]{}
	dec := riblt.Decoder[*testSymbol]{}

	nlocal := 0
	nremote := 384010
	//ncommon := 240000000
	ncommon := 0

	fmt.Println("preparing data")
	data := testSymbols(nlocal + nremote + ncommon)

	nextId := 0
	for i := 0; i < nlocal; i++ {
		dec.AddSymbol(&data[nextId])
		if nextId % 10000 == 0 {
			fmt.Println(nextId, "symbols inserted")
		}
		nextId += 1
	}
	for i := 0; i < nremote; i++ {
		enc.AddSymbol(&data[nextId])
		if nextId % 10000 == 0 {
			fmt.Println(nextId, "symbols inserted")
		}
		nextId += 1
	}
	for i := 0; i < ncommon; i++ {
		enc.AddSymbol(&data[nextId])
		dec.AddSymbol(&data[nextId])
		if nextId % 10000 == 0 {
			fmt.Println(nextId, "symbols inserted")
		}
		nextId += 1
	}

	ncw := 0
	start := time.Now()
	for {
		dec.AddCodedSymbol(enc.ProduceNextCodedSymbol())
		ncw += 1
		dec.TryDecode()
		if dec.Decoded() {
			break
		}
		if ncw % 10000 == 0 {
			fmt.Println(ncw, "codewords sent")
		}
	}
	dur := time.Now().Sub(start)
	fmt.Printf("%d codewords, %.2f overhead, %.2f seconds, %.2f diff/s\n", ncw, float64(ncw)/float64(nremote + nlocal), dur.Seconds(), float64(nremote + nlocal) / dur.Seconds())
}

