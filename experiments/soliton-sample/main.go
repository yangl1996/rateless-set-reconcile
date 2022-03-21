package main

import (
	"fmt"
	"github.com/yangl1996/soliton"
	"math/rand"
)

func main() {
    r1 := rand.New(rand.NewSource(0))
    s1 := soliton.NewSoliton(r1, 50)
	fmt.Println("\"Soliton\"")
	for i, p := range s1.PMF() {
		fmt.Println(i+1, p)
	}
	fmt.Println()
	fmt.Println()
	fmt.Println("\"Robust Soliton\"")
    s2 := soliton.NewRobustSoliton(r1, 50, 0.03, 0.5)
	for i, p := range s2.PMF() {
		fmt.Println(i+1, p)
	}
}
