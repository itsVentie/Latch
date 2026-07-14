package crypto_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"testing"

	"pqc-proxy/internal/crypto"
)

func TestSecureConnectionDataTransfer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	sharedKey := make([]byte, 32)
	if _, err := rand.Read(sharedKey); err != nil {
		t.Fatalf("Failed to generate static random key: %v", err)
	}

	message := make([]byte, 70*1024)
	if _, err := rand.Read(message); err != nil {
		t.Fatalf("Failed to generate test large message: %v", err)
	}

	errChan := make(chan error, 1)

	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		defer serverConn.Close()

		secureServer, err := crypto.NewSecureConn(serverConn, sharedKey)
		if err != nil {
			errChan <- err
			return
		}
		crypto.SetServerRoles(secureServer)

		receivedData := make([]byte, len(message))
		var total int
		for total < len(message) {
			n, readErr := secureServer.Read(receivedData[total:])
			if n > 0 {
				total += n
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				errChan <- readErr
				return
			}
		}

		if total != len(message) {
			errChan <- errors.New("did not receive the expected amount of data")
			return
		}

		if !bytes.Equal(receivedData, message) {
			errChan <- errors.New("data mismatch on server side")
			return
		}
		errChan <- nil
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Client failed to dial: %v", err)
	}
	defer clientConn.Close()

	secureClient, err := crypto.NewSecureConn(clientConn, sharedKey)
	if err != nil {
		t.Fatalf("Failed to wrap client conn: %v", err)
	}
	crypto.SetClientRoles(secureClient)

	_, err = secureClient.Write(message)
	if err != nil {
		t.Fatalf("Client write error: %v", err)
	}
	secureClient.Close()

	if serverErr := <-errChan; serverErr != nil {
		t.Fatalf("Server side error: %v", serverErr)
	}
}
