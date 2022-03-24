package main

import (
	"fmt"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func main() {
	r1 := rand.New(rand.NewSource(0))
    s := soliton.NewRobustSoliton(r1, 50, 0.03, 0.5)

	pmf := s.PMF()
	tot := 0.0
	var sd int
	for sd = len(pmf)-1; sd >= 0; sd-- {
		tot += pmf[sd]
		if tot > 0.001 {
			break
		}
	}
	loss := 0.0
	for i := sd; i < len(pmf); i++ {
		loss += float64(i) * pmf[i]
	}
	fmt.Println(loss)
}
