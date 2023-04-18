package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func main() {
	fmt.Println("# block size 200")
	fmt.Println("# absOverlap  relOverlap  absCost  relCost")
	for overlap := 0; overlap < 200; overlap++ {
		res := microbenchmarks.SimulateOneSenderOverlap(100, 200, overlap)
		fmt.Println(overlap, float64(overlap)/200, res, float64(res)/float64(200-overlap))
	}
}
