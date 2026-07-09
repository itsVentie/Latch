package network_test

import (
	"net"
	"testing"
)

func TestProxyPipe(t *testing.T) {
	client, server := net.Pipe()

	testData := []byte("hello pqc-tunnel")

	go func() {
		defer client.Close()
		_, _ = client.Write(testData)
	}()

	buf := make([]byte, 16)
	n, err := server.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(buf[:n]) != string(testData) {
		t.Errorf("Expected %s, got %s", testData, buf[:n])
	}
}
