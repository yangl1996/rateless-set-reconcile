package main

import (
//	"log"
	"net"
)

func main() {
	c1 := newController()
	c2 := newController()
	cn1, cn2 := net.Pipe()
	c1.newPeer <- cn1
	c2.newPeer <- cn2

	select{}
}
