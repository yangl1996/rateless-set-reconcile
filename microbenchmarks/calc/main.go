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
	fmt.Println("# efficiency assuming symmetry for block size 500")
	fmt.Println("# number of nodes   overhead")
	for nodes := 2; nodes <= 20; nodes++ {
		ncw := microbenchmarks.SimulateOneSenderOverlap(50, 5000, 5000-5000/nodes)
		fmt.Printf("%d & $%.2f$ \\\\\n", nodes, float64(ncw) / float64(5000/nodes))
	}
}
