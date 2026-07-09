package network_test

import (
	"io"
	"net"
	"testing"
	"time"

	"pqc-proxy/internal/crypto"
	"pqc-proxy/internal/network"
)

func TestEndToEndProxy(t *testing.T) {
	secret := "test-secret-key-32-bytes-long-hmac"

	pqcKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate PQC keys for test: %v", err)
	}

	target, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		t.Fatalf("Failed to bind target: %v", err)
	}
	defer target.Close()

	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	srv := network.NewServer("127.0.0.1:9090", "127.0.0.1:8000", pqcKeys, secret)
	go func() {
		_ = srv.Start()
	}()
	defer srv.Stop()

	cli := network.NewClient("127.0.0.1:3000", "127.0.0.1:9090", pqcKeys, secret)
	go func() {
		_ = cli.Start()
	}()
	defer cli.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "127.0.0.1:3000")
	if err != nil {
		t.Fatalf("Could not connect to client proxy: %v", err)
	}

	msg := []byte("quantum-safe-data")
	if _, err := conn.Write(msg); err != nil {
		conn.Close()
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	buf := make([]byte, len(msg))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		conn.Close()
		t.Fatalf("Failed to read from proxy: %v", err)
	}

	if string(buf) != string(msg) {
		t.Errorf("Expected %s, got %s", msg, buf)
	}

	conn.Close()
	time.Sleep(50 * time.Millisecond)
}
