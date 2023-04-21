package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func findMaxOverhead(blockSize int) (int, float64) {
	maxOverhead := 0.0
	maxOverheadOverlap := 0
	for common := 0; common <= blockSize; common++ {
		blk1 := microbenchmarks.SimulateOneSenderOverlap(100, blockSize, 0)
		blk2 := microbenchmarks.SimulateOneSenderOverlap(100, blockSize, common)
		overhead := float64(blk1 + blk2) / float64(blockSize + blockSize - common)
		if overhead > maxOverhead {
			maxOverhead = overhead
			maxOverheadOverlap = common
		}
	}
	return maxOverheadOverlap, maxOverhead
}

func main() {
	fmt.Println("# block size   overhead upperbound   maximizing overlap")
	blockSizes := []int{50, 100, 200, 500}
	for _, bs := range blockSizes {
		m, o := findMaxOverhead(bs)
		fmt.Println(bs, o, m)
	}
}
