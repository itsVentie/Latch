package network

import (
	"bytes"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"pqc-proxy/internal/network"
)

func TestSessionResumptionFlow(t *testing.T) {
	sessionTTL := 500 * time.Millisecond
	serverStore := network.NewSessionStore(sessionTTL)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind test listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()

				hello, err := network.ReadHandshakeHello(c)
				if err != nil {
					return
				}

				if len(hello.SessionID) > 0 {
					if cachedKey, ok := serverStore.Get(hello.SessionID); ok {
						response := &network.ServerHello{
							SessionResumed: true,
							SessionID:      hello.SessionID,
						}
						_ = network.WriteServerHello(c, response)
						_, _ = c.Write([]byte("resumed-data-transmission"))
						_ = cachedKey
						return
					}
				}

				newID, _ := serverStore.GenerateSessionID()
				generatedMasterKey := make([]byte, 32)
				_, _ = rand.Read(generatedMasterKey)
				serverStore.Put(newID, generatedMasterKey)

				response := &network.ServerHello{
					SessionResumed: false,
					SessionID:      newID,
					ServerEphPub:   []byte("test-pqc-ephemeral-pub"),
					ClientKEMCiph:  []byte("test-pqc-ciphertext"),
				}
				_ = network.WriteServerHello(c, response)
				_, _ = c.Write([]byte("full-handshake-data-transmission"))
			}(conn)
		}
	}()

	var savedSessionID []byte

	t.Run("First Connection: Full Handshake", func(t *testing.T) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		clientHello := &network.HandshakeHello{
			ClientEphPub: []byte("client-eph-pub"),
			ClientKEMPub: []byte("client-kem-pub"),
		}
		if err := network.WriteHandshakeHello(conn, clientHello); err != nil {
			t.Fatalf("Failed to write client hello: %v", err)
		}

		resp, err := network.ReadServerHello(conn)
		if err != nil {
			t.Fatalf("Failed to read server hello: %v", err)
		}

		if resp.SessionResumed {
			t.Error("Expected full handshake, but server reported session resumption")
		}

		if len(resp.SessionID) == 0 {
			t.Error("Server did not return a session ID")
		}
		savedSessionID = resp.SessionID
	})

	t.Run("Second Connection: Fast Resumption", func(t *testing.T) {
		if len(savedSessionID) == 0 {
			t.Fatal("No session ID saved from first step")
		}

		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		clientHello := &network.HandshakeHello{
			SessionID: savedSessionID,
		}
		if err := network.WriteHandshakeHello(conn, clientHello); err != nil {
			t.Fatalf("Failed to write client hello: %v", err)
		}

		resp, err := network.ReadServerHello(conn)
		if err != nil {
			t.Fatalf("Failed to read server hello: %v", err)
		}

		if !resp.SessionResumed {
			t.Error("Expected session resumption, but server performed full handshake")
		}

		if !bytes.Equal(resp.SessionID, savedSessionID) {
			t.Error("Session ID mismatch on resumption")
		}

		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		if !bytes.Contains(buf[:n], []byte("resumed-data")) {
			t.Errorf("Unexpected data from server: %s", string(buf[:n]))
		}
	})

	t.Run("Third Connection: Session Expiry (Fallback to Full Handshake)", func(t *testing.T) {
		time.Sleep(sessionTTL + 50*time.Millisecond)

		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		clientHello := &network.HandshakeHello{
			SessionID: savedSessionID,
		}
		if err := network.WriteHandshakeHello(conn, clientHello); err != nil {
			t.Fatalf("Failed to write client hello: %v", err)
		}

		resp, err := network.ReadServerHello(conn)
		if err != nil {
			t.Fatalf("Failed to read server hello: %v", err)
		}

		if resp.SessionResumed {
			t.Error("Server resumed expired session")
		}

		if bytes.Equal(resp.SessionID, savedSessionID) {
			t.Error("Server returned the same expired session ID")
		}
	})
}
