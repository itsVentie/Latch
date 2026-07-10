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

type Server struct {
	listenAddr string
	targetAddr string
	listener   net.Listener
	pqcKeys    *crypto.PQCKeyPair
	TLSConfig  *tls.Config
}

func NewServer(listenAddr, targetAddr string, keys *crypto.PQCKeyPair, tlsConfig *tls.Config) *Server {
	return &Server{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
		pqcKeys:    keys,
		TLSConfig:  tlsConfig,
	}
}

func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("server failed to listen: %w", err)
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			slog.Error("Failed to accept server connection", "error", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()
	remoteAddr := clientConn.RemoteAddr().String()

	if s.TLSConfig != nil {
		tlsConn := tls.Server(clientConn, s.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			slog.Error("Server mTLS handshake failed", "remote_addr", remoteAddr, "error", err)
			return
		}
		clientConn = tlsConn
	}

	slog.Debug("Handling incoming server connection", "remote_addr", remoteAddr)

	clientInceptionBlob := make([]byte, crypto.ClientInceptionSize)
	if _, err := io.ReadFull(clientConn, clientInceptionBlob); err != nil {
		slog.Error("Failed to read server inception blob", "remote_addr", remoteAddr, "error", err)
		return
	}

	masterKey, responseBlob, err := crypto.ServerHandleInception(clientInceptionBlob)
	if err != nil {
		slog.Error("Server PQC inception handling failed", "remote_addr", remoteAddr, "error", err)
		return
	}

	if _, err := clientConn.Write(responseBlob); err != nil {
		slog.Error("Failed to write server handshake response", "remote_addr", remoteAddr, "error", err)
		return
	}

	secureClientConn, err := crypto.NewSecureConn(clientConn, masterKey)
	if err != nil {
		slog.Error("Failed to establish secure server connection layer", "remote_addr", remoteAddr, "error", err)
		return
	}
	crypto.SetServerRoles(secureClientConn)

	slog.Info("Server hybrid handshake completed successfully", "remote_addr", remoteAddr)

	targetConn, err := net.Dial("tcp", s.targetAddr)
	if err != nil {
		slog.Error("Server failed to dial target backend", "target_addr", s.targetAddr, "remote_addr", remoteAddr, "error", err)
		return
	}
	defer targetConn.Close()

	slog.Debug("Server connected to target backend", "target_addr", s.targetAddr)

	errChan := make(chan error, 2)
	go func() { errChan <- proxyPipe(secureClientConn, targetConn) }()
	go func() { errChan <- proxyPipe(targetConn, secureClientConn) }()

	if pipeErr := <-errChan; pipeErr != nil && !errors.Is(pipeErr, io.EOF) {
		slog.Error("Server tunnel pipe closed with error", "remote_addr", remoteAddr, "error", pipeErr)
	} else {
		slog.Info("Server tunnel pipe connection closed cleanly", "remote_addr", remoteAddr)
	}
}

func proxyPipe(dst io.Writer, src io.Reader) error {
	bufPtr := GetBuffer()
	defer PutBuffer(bufPtr)
	buf := *bufPtr
	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
