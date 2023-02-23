package lt

import (
	"github.com/yangl1996/soliton"
	"github.com/dchest/siphash"
	"testing"
	"encoding/binary"
	"math/rand"
)

var testSalt = [SaltSize]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
var hasher = siphash.New(testSalt[:])

const simpleDataSize = 128
type simpleData [simpleDataSize]byte

func (d *simpleData) XOR(t2 *simpleData) *simpleData{
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

func (d *simpleData) Equals(t2 *simpleData) bool {
	for i := 0; i < simpleDataSize; i++ {
		if d[i] != t2[i] {
			return false
		}
	}
	return true
}

func newSimpleData(i uint64) *simpleData {
	data := simpleData{}
	binary.LittleEndian.PutUint64(data[0:8], i)
	return &data
}

func TestEncodeAndDecode(t *testing.T) {
	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(0)), 500, 0.03, 0.5)
	e := NewEncoder[*simpleData](rand.New(rand.NewSource(0)), testSalt, dist, 500)
	for i := 0; i < 500; i++ {
		tx := NewTransaction[*simpleData](newSimpleData(uint64(i)))
		e.AddTransaction(tx)
	}
	dec := NewDecoder[*simpleData](testSalt, 100000)
	ncw := 0
	ndec := 0
	for ndec < 500 {
		c := e.ProduceCodeword()
		_, newtx := dec.AddCodeword(c)
		ncw += 1
		ndec += len(newtx)
	}
	for _, tx := range e.window {
		_, there := dec.receivedTransactions[tx.saltedHash]
		if !there {
			t.Error("missing transaction in the decoder")
		}
	}
	t.Logf("%d codewords until fully decoded", ncw)
}

