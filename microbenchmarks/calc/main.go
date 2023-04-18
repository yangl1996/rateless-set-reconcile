package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func main() {
	fmt.Println("# degree   overhead upperbound")
	for deg := 1; deg < 32; deg++ {
		res := microbenchmarks.SimulateOneSenderOverlap(100, 200, 200-200/deg)
		fmt.Println(deg, float64(res)/float64(200/deg))
	}
}
