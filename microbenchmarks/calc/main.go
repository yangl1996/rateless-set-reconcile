package main

import (
	"fmt"
	"github.com/yangl1996/rateless-set-reconcile/microbenchmarks"
)

func findMaxOverhead(blockSize int, numNodes int) ([]float64, []int) {
	totTx := blockSize
	totCw := microbenchmarks.SimulateOneSenderOverlap(100, blockSize, 0)

	maxOverhead := []float64{}
	maximizingOverlap := []int{}
	for node := 1; node < numNodes; node++ {
		overhead := 0.0
		overlap := 0
		cw := 0
		for common := 0; common <= blockSize; common++ {
			blk := microbenchmarks.SimulateOneSenderOverlap(100, blockSize, common)
			thisOverhead := float64(totCw + blk) / float64(totTx + blockSize - common)
			if thisOverhead > overhead {
				overhead = thisOverhead
				overlap = common
				cw = blk
			}
		}
		maxOverhead = append(maxOverhead, overhead)
		maximizingOverlap = append(maximizingOverlap, overlap)
		totTx += blockSize - overlap
		totCw += cw
	}
	return maxOverhead, maximizingOverlap
}

func main() {
	fmt.Println("# worst efficiency for 2 overlapping nodes at different block sizes")
	fmt.Println("# block size   overhead upperbound   maximizing overlap")
	blockSizes := []int{50, 100, 200, 500}
	for _, bs := range blockSizes {
		o, m := findMaxOverhead(bs, 2)
		fmt.Println(bs, o[0], m[0])
	}
	fmt.Println("# efficiency assuming symmetry for block size 200")
	fmt.Println("# number of nodes   overhead")
	for nodes := 2; nodes < 16; nodes++ {
		ncw := microbenchmarks.SimulateOneSenderOverlap(100, 200, 200-200/nodes)
		fmt.Println(nodes, float64(ncw) / float64(200/nodes))
	}
	fmt.Println("# efficiency for block size 100 as there are more overlapping nodes")
	fmt.Println("# n-th node   overhead upperbound   maximizing overlap")
	o, m := findMaxOverhead(100, 10)
	for nodes := 2; nodes <= 10; nodes++ {
		fmt.Println(nodes, o[nodes-2], m[nodes-2])
	}
}
