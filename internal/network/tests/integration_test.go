package network

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestEndToEndProxy(t *testing.T) {
	target, _ := net.Listen("tcp", "127.0.0.1:8000")
	go func() {
		conn, _ := target.Accept()
		io.Copy(conn, conn)
		conn.Close()
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "127.0.0.1:3000")
	if err != nil {
		t.Fatalf("Could not connect: %v", err)
	}

	msg := []byte("quantum-safe-data")
	conn.Write(msg)

	buf := make([]byte, len(msg))
	conn.Read(buf)

	if string(buf) != string(msg) {
		t.Errorf("Expected %s, got %s", msg, buf)
	}
	conn.Close()
}
