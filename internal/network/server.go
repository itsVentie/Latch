package network

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"pqc-proxy/internal/crypto"
)

type Server struct {
	listenAddr   string
	targetAddr   string
	listener     net.Listener
	pqcKeys      *crypto.PQCKeyPair
	TLSConfig    *tls.Config
	sessionStore *SessionStore
}

func NewServer(listenAddr, targetAddr string, keys *crypto.PQCKeyPair, tlsConfig *tls.Config) *Server {
	return &Server{
		listenAddr:   listenAddr,
		targetAddr:   targetAddr,
		pqcKeys:      keys,
		TLSConfig:    tlsConfig,
		sessionStore: NewSessionStore(24 * time.Hour),
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

	hello, err := ReadHandshakeHello(clientConn)
	if err != nil {
		slog.Error("Failed to read handshake hello from client", "remote_addr", remoteAddr, "error", err)
		return
	}

	var masterKey []byte
	var sessionID []byte
	resumed := false

	if len(hello.SessionID) > 0 {
		if cachedKey, ok := s.sessionStore.Get(hello.SessionID); ok {
			masterKey = cachedKey
			sessionID = hello.SessionID
			resumed = true
			slog.Info("Session resumption requested and approved", "remote_addr", remoteAddr, "session_id", fmt.Sprintf("%x", sessionID[:8]))

			response := &ServerHello{
				SessionResumed: true,
				SessionID:      sessionID,
			}
			if err := WriteServerHello(clientConn, response); err != nil {
				slog.Error("Failed to write server hello resumption response", "remote_addr", remoteAddr, "error", err)
				return
			}
		}
	}

	if !resumed {
		if len(hello.ClientEphPub) == 0 || len(hello.ClientKEMPub) == 0 {
			slog.Error("Invalid handshake hello for full handshake", "remote_addr", remoteAddr)
			return
		}

		clientInceptionBlob := append(hello.ClientEphPub, hello.ClientKEMPub...)
		var responseBlob []byte
		masterKey, responseBlob, err = crypto.ServerHandleInception(clientInceptionBlob)
		if err != nil {
			slog.Error("Server PQC inception handling failed", "remote_addr", remoteAddr, "error", err)
			return
		}

		sessionID, err = s.sessionStore.GenerateSessionID()
		if err != nil {
			slog.Error("Failed to generate session ID", "remote_addr", remoteAddr, "error", err)
			return
		}

		s.sessionStore.Put(sessionID, masterKey)

		response := &ServerHello{
			SessionResumed: false,
			SessionID:      sessionID,
			ServerEphPub:   responseBlob[:32],
			ClientKEMCiph:  responseBlob[32:],
		}
		if err := WriteServerHello(clientConn, response); err != nil {
			slog.Error("Failed to write server hello full handshake response", "remote_addr", remoteAddr, "error", err)
			return
		}
	}

	secureClientConn, err := crypto.NewSecureConn(clientConn, masterKey)
	if err != nil {
		slog.Error("Failed to establish secure server connection layer", "remote_addr", remoteAddr, "error", err)
		return
	}
	crypto.SetServerRoles(secureClientConn)

	slog.Info("Server hybrid handshake completed successfully", "remote_addr", remoteAddr, "resumed", resumed)

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
