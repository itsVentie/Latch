package network

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"pqc-proxy/internal/crypto"
)

type Client struct {
	listenAddr      string
	serverAddr      string
	tcpListener     net.Listener
	udpListener     *net.UDPConn
	pqcKeys         *crypto.PQCKeyPair
	TLSConfig       *tls.Config
	mu              sync.Mutex
	cachedSessionID []byte
	cachedMasterKey []byte
	running         bool
	handshakeDone   bool
}

func NewClient(listenAddr, serverAddr string, keys *crypto.PQCKeyPair, tlsConfig *tls.Config) *Client {
	return &Client{
		listenAddr: listenAddr,
		serverAddr: serverAddr,
		pqcKeys:    keys,
		TLSConfig:  tlsConfig,
	}
}

func (c *Client) Start() error {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	var err error
	c.tcpListener, err = net.Listen("tcp", c.listenAddr)
	if err != nil {
		return fmt.Errorf("client failed to listen TCP: %w", err)
	}

	uAddr, err := net.ResolveUDPAddr("udp", c.listenAddr)
	if err != nil {
		c.tcpListener.Close()
		return fmt.Errorf("client failed to resolve UDP address: %w", err)
	}
	c.udpListener, err = net.ListenUDP("udp", uAddr)
	if err != nil {
		c.tcpListener.Close()
		return fmt.Errorf("client failed to listen UDP: %w", err)
	}

	go c.listenTCP()
	go c.listenUDP()

	return nil
}

func (c *Client) Stop() {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	if c.tcpListener != nil {
		c.tcpListener.Close()
	}
	if c.udpListener != nil {
		c.udpListener.Close()
	}
}

func (c *Client) listenTCP() {
	for {
		conn, err := c.tcpListener.Accept()
		if err != nil {
			return
		}
		c.mu.Lock()
		done := c.handshakeDone
		c.mu.Unlock()

		if !done {
			go c.triggerHandshakeAndDelegation(conn.RemoteAddr().String(), "TCP")
			conn.Close()
		} else {
			conn.Close()
		}
	}
}

func (c *Client) listenUDP() {
	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := c.udpListener.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}

		c.mu.Lock()
		done := c.handshakeDone
		c.mu.Unlock()

		if !done {
			go c.triggerHandshakeAndDelegation(remoteAddr.String(), "UDP")
		} else {
		}
	}
}

func (c *Client) triggerHandshakeAndDelegation(srcAddr string, protocol string) {
	c.mu.Lock()
	if c.handshakeDone {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	slog.Debug("Handling incoming client traffic trigger", "addr", srcAddr, "protocol", protocol)

	serverConn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		slog.Error("Client failed to dial remote PQC server", "server_addr", c.serverAddr, "error", err)
		return
	}
	defer serverConn.Close()

	if c.TLSConfig != nil {
		tlsConn := tls.Client(serverConn, c.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			slog.Error("Client mTLS handshake failed", "error", err)
			return
		}
		serverConn = tlsConn
	}

	c.mu.Lock()
	sessionID := c.cachedSessionID
	cachedMasterKey := c.cachedMasterKey
	c.mu.Unlock()

	var masterKey []byte
	var activeSessionID []byte
	resumed := false

	if len(sessionID) > 0 {
		hello := &HandshakeHello{SessionID: sessionID}
		if err := WriteHandshakeHello(serverConn, hello); err == nil {
			resp, err := ReadServerHello(serverConn)
			if err == nil && resp.SessionResumed {
				slog.Info("Session resumption completed successfully", "protocol", protocol)
				masterKey = cachedMasterKey
				activeSessionID = sessionID
				resumed = true
			}
		}
	}

	if !resumed {
		ecdhPriv, mlkemPriv, clientBlob, err := crypto.GenerateClientInception()
		if err != nil {
			slog.Error("Client failed to generate PQC payload", "error", err)
			return
		}

		hello := &HandshakeHello{
			ClientEphPub: clientBlob[:32],
			ClientKEMPub: clientBlob[32:],
		}

		if err := WriteHandshakeHello(serverConn, hello); err != nil {
			slog.Error("Client failed to write handshake hello", "error", err)
			return
		}

		resp, err := ReadServerHello(serverConn)
		if err != nil {
			slog.Error("Client failed to read server response", "error", err)
			return
		}

		serverResponseBlob := append(resp.ServerEphPub, resp.ClientKEMCiph...)
		masterKey, err = crypto.ClientHandleResponse(ecdhPriv, mlkemPriv, serverResponseBlob)
		if err != nil {
			slog.Error("Client failed to process response", "error", err)
			return
		}

		activeSessionID = resp.SessionID
		c.mu.Lock()
		c.cachedSessionID = resp.SessionID
		c.cachedMasterKey = masterKey
		c.mu.Unlock()
	}

	slog.Info("Client handshake completed successfully", "protocol", protocol, "resumed", resumed)

	c.mu.Lock()
	c.handshakeDone = true
	c.mu.Unlock()

	if protocol == "UDP" && c.udpListener != nil {
		c.udpListener.Close()
	} else if protocol == "TCP" && c.tcpListener != nil {
		c.tcpListener.Close()
	}

	err = ForwardContextToDataplane(fmt.Sprintf("%x", activeSessionID), masterKey, c.listenAddr, c.serverAddr, protocol)
	if err != nil {
		slog.Error("Client failed to forward context to rust dataplane", "error", err)
		return
	}

	slog.Info("Client control plane delegated processing to rust dataplane", "protocol", protocol)
}
