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

func BenchmarkProxyPipe(b *testing.B) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := make([]byte, 16384)
	for i := range data {
		data[i] = 'A'
	}

	b.ReportAllocs()
	b.ResetTimer()

	go func() {
		buf := make([]byte, 16384)
		for {
			_, err := server.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	for i := 0; i < b.N; i++ {
		_, err := client.Write(data)
		if err != nil {
			b.Fatalf("Benchmark write failed: %v", err)
		}
	}
}
