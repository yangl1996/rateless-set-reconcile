package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func main() {
	fmt.Println("# block size 200")
	fmt.Println("# absFresh relFresh absCost  relCost")
	for fresh := 0; fresh < 200; fresh++ {
		res := microbenchmarks.SimulateOneSenderOverlap(100, 200, 200-fresh)
		fmt.Println(fresh, float64(fresh)/200, res, float64(res)/float64(fresh))
	}
}
