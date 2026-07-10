package network

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"pqc-proxy/internal/crypto"
)

type Client struct {
	listenAddr string
	serverAddr string
	listener   net.Listener
	pqcKeys    *crypto.PQCKeyPair
	TLSConfig  *tls.Config
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
	var err error
	c.listener, err = net.Listen("tcp", c.listenAddr)
	if err != nil {
		return fmt.Errorf("client failed to listen: %w", err)
	}

	for {
		localConn, err := c.listener.Accept()
		if err != nil {
			slog.Error("Failed to accept local client connection", "error", err)
			return nil
		}

		go c.handleConnection(localConn)
	}
}

func (c *Client) Stop() {
	if c.listener != nil {
		c.listener.Close()
	}
}

func (c *Client) handleConnection(localConn net.Conn) {
	defer localConn.Close()
	localAddr := localConn.RemoteAddr().String()

	slog.Debug("Handling incoming local client connection", "local_addr", localAddr)

	serverConn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		slog.Error("Client failed to dial remote PQC server", "server_addr", c.serverAddr, "local_addr", localAddr, "error", err)
		return
	}
	defer serverConn.Close()

	if c.TLSConfig != nil {
		tlsConn := tls.Client(serverConn, c.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			slog.Error("Client mTLS handshake failed", "local_addr", localAddr, "error", err)
			return
		}
		serverConn = tlsConn
	}

	ecdhPriv, mlkemPriv, clientBlob, err := crypto.GenerateClientInception()
	if err != nil {
		slog.Error("Client failed to generate PQC inception payload", "local_addr", localAddr, "error", err)
		return
	}

	if _, err := serverConn.Write(clientBlob); err != nil {
		slog.Error("Client failed to write inception payload to server", "local_addr", localAddr, "error", err)
		return
	}

	serverResponseBlob := make([]byte, crypto.ServerResponseSize)
	if _, err := io.ReadFull(serverConn, serverResponseBlob); err != nil {
		slog.Error("Client failed to read server handshake response", "local_addr", localAddr, "error", err)
		return
	}

	masterKey, err := crypto.ClientHandleResponse(ecdhPriv, mlkemPriv, serverResponseBlob)
	if err != nil {
		slog.Error("Client failed to process server handshake response", "local_addr", localAddr, "error", err)
		return
	}

	secureServerConn, err := crypto.NewSecureConn(serverConn, masterKey)
	if err != nil {
		slog.Error("Client failed to wrap secure connection layer", "local_addr", localAddr, "error", err)
		return
	}
	crypto.SetClientRoles(secureServerConn)

	slog.Info("Client hybrid handshake completed successfully", "local_addr", localAddr)

	errChan := make(chan error, 2)
	go func() { errChan <- proxyPipe(localConn, secureServerConn) }()
	go func() { errChan <- proxyPipe(secureServerConn, localConn) }()

	if pipeErr := <-errChan; pipeErr != nil && !errors.Is(pipeErr, io.EOF) {
		slog.Error("Client tunnel pipe closed with error", "local_addr", localAddr, "error", pipeErr)
	} else {
		slog.Info("Client tunnel pipe connection closed cleanly", "local_addr", localAddr)
	}
}
