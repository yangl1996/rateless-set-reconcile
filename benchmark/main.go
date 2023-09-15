package main

import (
	"math/rand"
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

func testSymbols(n int, from int) []testSymbol {
	data := make([]testSymbol, n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint64(data[i][0:8], uint64(i)+uint64(from))
	}
	return data
}

func randomTestSymbols(n int, from int) []testSymbol {
	src := rand.Perm(n)
	data := make([]testSymbol, n)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint64(data[i][0:8], uint64(src[i])+uint64(from))
	}
	return data
}

func hashSymbols(in []testSymbol) []riblt.HashedSymbol[testSymbol] {
	res := []riblt.HashedSymbol[testSymbol]{}
	for _, v := range in {
		res = append(res, riblt.HashedSymbol[testSymbol]{v, v.Hash()})
	}
	return res
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
	nremote := *diff / 2
	ncommon := *set - *diff/2

	totalCw := 0

	var encDur, decDur time.Duration
	for testIdx := 0; testIdx < *test; testIdx += 1 {
		symbolBegin := rand.Int()
		diffData := randomTestSymbols(*diff, symbolBegin)
		hashedDiff := hashSymbols(diffData)
		commonData := testSymbols(ncommon, *diff+symbolBegin)
		//hashedCommon := hashSymbols(commonData)
		// probe number of symbols
		ncw := 0
		{
			enc := riblt.Encoder[testSymbol]{}
			dec := riblt.Decoder[testSymbol]{}
			for i := 0; i < nlocal; i++ {
				dec.AddHashedSymbol(hashedDiff[i])
			}
			for i := nlocal; i < nlocal+nremote; i++ {
				enc.AddHashedSymbol(hashedDiff[i])
			}
			for {
				dec.AddCodedSymbol(enc.ProduceNextCodedSymbol())
				ncw += 1
				dec.TryDecode()
				if dec.Decoded() {
					break
				}
			}
			totalCw += ncw
		}

		// benchmark encode
		{
			var codewords riblt.Sketch[testSymbol] 
			codewords = make([]riblt.CodedSymbol[testSymbol], ncw)
			start := time.Now()
			for i := nlocal; i < nlocal+nremote; i++ {
				codewords.AddSymbol(diffData[i])
			}
			for i := 0; i < ncommon; i++ {
				codewords.AddSymbol(commonData[i])
			}
			dur := time.Now().Sub(start)
			encDur += dur
		}

		// benchmark decode
		{
			enc := riblt.Encoder[testSymbol]{}
			dec := riblt.Decoder[testSymbol]{}
			// first fill the decoder
			for i := 0; i < nlocal; i++ {
				dec.AddHashedSymbol(hashedDiff[i])
			}
			for i := nlocal; i < nlocal+nremote; i++ {
				enc.AddHashedSymbol(hashedDiff[i])
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
	}

	fmt.Printf("%.2f overhead, enc %.2f diff/s, dec %.2f diff/s\n", float64(totalCw)/float64(*test)/float64(*diff), float64(*diff)*float64(*test) / encDur.Seconds(), float64(*test)*float64(*diff)/decDur.Seconds())
}

