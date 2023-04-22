package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func main() {
	fmt.Println("# block size 200, overlap 100, two senders")
	fmt.Println("# sender1Cw  absCost  relCost")
	for sender1 := 0; sender1 < 400; sender1++ {
		res := microbenchmarks.SimulateTwoSendersOverlap(100, 200, 100, sender1)
		if res != -1 {
			fmt.Println(sender1, res, float64(res)/float64(300))
		}
	}
}
