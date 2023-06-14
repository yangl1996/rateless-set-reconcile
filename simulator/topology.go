package main

import (
	"fmt"
	"time"
	"os"
	"bufio"
)

type connection struct {
	a int
	b int
	delay time.Duration
}

func loadTopology(path string) ([]connection, int) {
	res := []connection{}
	maxIdx := 0
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	s := bufio.NewScanner(file)
	for s.Scan() {
		var a, b, d int
		n, err := fmt.Sscanf(s.Text(), "%d,%d,%d", &a, &b, &d)
		if err != nil {
			panic(err)
		}
		if n == 3 {
			res = append(res, connection{a, b, time.Duration(d) * time.Millisecond})
			if a > maxIdx {
				maxIdx = a
			}
			if b > maxIdx {
				maxIdx = b
			}
		}
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
	return res, maxIdx+1
}
