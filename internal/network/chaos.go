package network

import (
	"math/rand"
	"net"
	"time"
)

func InjectChaos(conn net.Conn) {
	rand.Seed(time.Now().UnixNano())

	threshold := rand.Intn(100)

	if threshold < 5 {
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	} else if threshold < 15 {
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
}
