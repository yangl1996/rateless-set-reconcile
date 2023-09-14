package main

import (
	"encoding/binary"
	"github.com/dchest/siphash"
	"github.com/yangl1996/rateless-set-reconcile/riblt"
	"time"
	"fmt"
	"flag"
	"unsafe"
)

const testSymbolSize = 8

type testSymbol [testSymbolSize]byte

func (d testSymbol) XOR(t2 testSymbol) testSymbol {
	dw := (*uint64)(unsafe.Pointer(&d))
	t2w := (*uint64)(unsafe.Pointer(&t2))
	*dw = *dw ^ *t2w
	return d
}

func (d testSymbol) Hash() uint64 {
	return siphash.Hash(567, 890, d[:])
}

func testSymbols(n int) []testSymbol {
	fmt.Println("allocating memory")
	data := make([]testSymbol, n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(data[i][0:4], uint32(i))
	}
	return data
}

func main() {
	diff := flag.Int("d", 0, "number of differences")
	set := flag.Int("s", 0, "size of set")
	test := flag.Int("n", 100, "number of tests")
	flag.Parse()
	if *diff % 2 != 0 {
		panic("diff not an even number")
	}
	nlocal := *diff / 2
	//nremote := 384010
	nremote := *diff / 2
	//ncommon := 240000000
	ncommon := *set - *diff/2

	fmt.Println("preparing data")
	data := testSymbols(nlocal + nremote + ncommon)

	fmt.Println("determining number of symbols to generate")
	ncw := 0
	{
		enc := riblt.Encoder[testSymbol]{}
		dec := riblt.Decoder[testSymbol]{}
		nextId := 0
		for i := 0; i < nlocal; i++ {
			dec.AddSymbol(data[nextId])
			nextId += 1
		}
		for i := 0; i < nremote; i++ {
			enc.AddSymbol(data[nextId])
			nextId += 1
		}
		for i := 0; i < ncommon; i++ {
			enc.AddSymbol(data[nextId])
			dec.AddSymbol(data[nextId])
			nextId += 1
		}

		for {
			dec.AddCodedSymbol(enc.ProduceNextCodedSymbol())
			ncw += 1
			dec.TryDecode()
			if dec.Decoded() {
				break
			}
		}
	}
	fmt.Println("running")

	var encDur, decDur time.Duration
	for testIdx := 0; testIdx < *test; testIdx++ {
		enc := riblt.Encoder[testSymbol]{}
		nextId := 0
		for i := 0; i < nremote; i++ {
			enc.AddSymbol(data[nextId])
			nextId += 1
		}
		for i := 0; i < ncommon; i++ {
			enc.AddSymbol(data[nextId])
			nextId += 1
		}

		start := time.Now()
		for i := 0; i < ncw; i++ {
			enc.ProduceNextCodedSymbol()
		}
		dur := time.Now().Sub(start)
		encDur += dur
	}
	for testIdx := 0; testIdx < *test; testIdx++ {
		enc := riblt.Encoder[testSymbol]{}
		dec := riblt.Decoder[testSymbol]{}
		nextId := 0
		for i := 0; i < nlocal; i++ {
			dec.AddSymbol(data[nextId])
			nextId += 1
		}
		for i := 0; i < nremote; i++ {
			enc.AddSymbol(data[nextId])
			nextId += 1
		}
		for i := 0; i < ncommon; i++ {
			enc.AddSymbol(data[nextId])
			dec.AddSymbol(data[nextId])
			nextId += 1
		}

		for i := 0; i < ncw; i++ {
			dec.AddCodedSymbol(enc.ProduceNextCodedSymbol())
		}
		start := time.Now()
		dec.TryDecode()
		dur := time.Now().Sub(start)
		decDur += dur
		if !dec.Decoded() {
			panic("fail to decode")
		}
	}
	fmt.Printf("%d codewords, %.2f overhead, enc %.2f diff/s, dec %.2f diff/s\n", ncw, float64(ncw)/float64(nremote + nlocal), float64(nremote + nlocal)*float64(*test) / encDur.Seconds(), float64(*test)*float64(nremote+nlocal)/decDur.Seconds())
}

