package network

import (
	"crypto/rand"
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
	listenAddr string
	targetAddr string
	listener   net.Listener
	pqcKeys    *crypto.PQCKeyPair
	TLSConfig  *tls.Config
	tdkMgr     *crypto.TdkManager
}

func NewServer(listenAddr, targetAddr string, keys *crypto.PQCKeyPair, tlsConfig *tls.Config) *Server {
	tdkMgr, err := crypto.NewTdkManager(4 * time.Hour)
	if err != nil {
		slog.Error("Failed to initialize TDK manager", "error", err)
		return nil
	}

	return &Server{
		listenAddr: listenAddr,
		targetAddr: targetAddr,
		pqcKeys:    keys,
		TLSConfig:  tlsConfig,
		tdkMgr:     tdkMgr,
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
	if s.tdkMgr != nil {
		s.tdkMgr.Stop()
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

	if len(hello.SessionTicket) > 0 {
		state, needsReissue, err := s.tdkMgr.ValidateTicket(hello.SessionTicket)
		if err == nil {
			masterKey = state.MasterKey
			sessionID = state.ID
			resumed = true
			slog.Info("Session resumption requested and approved via ticket", "remote_addr", remoteAddr, "session_id", fmt.Sprintf("%x", sessionID[:8]))

			var newTicket []byte
			if needsReissue {
				slog.Debug("Ticket validated via previous TDK key, issuing fresh ticket", "remote_addr", remoteAddr)
				newTicket, _ = s.tdkMgr.IssueTicket(state)
			}

			response := &ServerHello{
				SessionResumed: true,
				SessionID:      sessionID,
				NewTicket:      newTicket,
			}
			if err := WriteServerHello(clientConn, response); err != nil {
				slog.Error("Failed to write server hello resumption response", "remote_addr", remoteAddr, "error", err)
				return
			}
		} else {
			slog.Debug("Ticket validation failed, falling back to full handshake", "remote_addr", remoteAddr, "error", err)
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

		sessionID = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, sessionID); err != nil {
			slog.Error("Failed to generate session ID", "remote_addr", remoteAddr, "error", err)
			return
		}

		state := &crypto.SessionState{
			ID:        sessionID,
			MasterKey: masterKey,
			CreatedAt: time.Now(),
		}

		ticket, err := s.tdkMgr.IssueTicket(state)
		if err != nil {
			slog.Error("Failed to issue session ticket", "remote_addr", remoteAddr, "error", err)
			return
		}

		response := &ServerHello{
			SessionResumed: false,
			SessionID:      sessionID,
			ServerEphPub:   responseBlob[:32],
			ClientKEMCiph:  responseBlob[32:],
			NewTicket:      ticket,
		}
		if err := WriteServerHello(clientConn, response); err != nil {
			slog.Error("Failed to write server hello full handshake response", "remote_addr", remoteAddr, "error", err)
			return
		}
	}

	slog.Info("Server hybrid handshake completed successfully", "remote_addr", remoteAddr, "resumed", resumed)

	err = ForwardContextToDataplane(fmt.Sprintf("%x", sessionID), masterKey, s.listenAddr, s.targetAddr, "UDP")
	if err != nil {
		slog.Error("Server failed to forward context to rust dataplane", "error", err)
		return
	}

	slog.Info("Server control plane delegated processing to rust dataplane", "remote_addr", remoteAddr)
}

func proxyPipe(dst io.Writer, src io.Reader) error {
	bufPtr := GetBuffer()
	defer PutBuffer(bufPtr)
	buf := *bufPtr
	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
