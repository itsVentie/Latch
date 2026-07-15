package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pqc-proxy/internal/config"
	"pqc-proxy/internal/crypto"
	"pqc-proxy/internal/logger"
	"pqc-proxy/internal/metrics"
	"pqc-proxy/internal/network"
)

func main() {
	cfg := config.Load()

	cfg.Debug = true
	logger.Init(cfg.Debug)

	if cfg.Mode != "client" && cfg.Mode != "server" {
		slog.Error("Invalid execution mode", "mode", cfg.Mode, "expected", "client|server")
		os.Exit(1)
	}

	if cfg.ListenAddr == "" || cfg.TargetAddr == "" {
		slog.Error("Missing required network parameters", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
		os.Exit(1)
	}

	slog.Info("Initializing pqc-proxy", "mode", cfg.Mode, "version", "1.1.0", "debug", cfg.Debug)

	pqcKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		slog.Error("Failed to initialize PQC keys", "error", err)
		os.Exit(1)
	}
	slog.Info("PQC keys generated successfully")

	metricsPort := cfg.MetricsAddr
	if cfg.Mode == "client" {
		metricsPort = ":2113"
	}

	metrics.Init()
	go func() {
		slog.Info("Starting Prometheus metrics server", "addr", metricsPort)
		if err := metrics.StartServer(metricsPort); err != nil {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	slog.Debug("Generating dynamic TLS configuration", "isServer", cfg.Mode == "server")
	tlsConfig, err := generateInsecuremTLSConfig(cfg.Mode == "server")
	if err != nil {
		slog.Error("Failed to generate default mTLS configurations", "error", err)
		os.Exit(1)
	}

	if cfg.Mode == "server" {
		srv := network.NewServer(cfg.ListenAddr, cfg.TargetAddr, pqcKeys, tlsConfig)
		slog.Info("Starting PQC SERVER (mTLS secured)", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
		go func() {
			if err := srv.Start(); err != nil {
				slog.Error("Server runtime error", "error", err)
				os.Exit(1)
			}
		}()
		<-sigChan
		srv.Stop()
	} else {
		cli := network.NewClient(cfg.ListenAddr, cfg.TargetAddr, pqcKeys, tlsConfig)
		slog.Info("Starting PQC CLIENT (mTLS secured)", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
		go func() {
			if err := cli.Start(); err != nil {
				slog.Error("Client runtime error", "error", err)
				os.Exit(1)
			}
		}()
		<-sigChan
		cli.Stop()
	}
	slog.Info("Application stopped cleanly")
}

func generateInsecuremTLSConfig(isServer bool) (*tls.Config, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		slog.Error("Failed to generate private key for TLS", "error", err)
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"PQC-Proxy-Dev"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		slog.Error("Failed to sign self-signed TLS certificate template", "error", err)
		return nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	certPool := x509.NewCertPool()
	parsedCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		slog.Error("Failed to parse newly generated certificate bytes", "error", err)
		return nil, err
	}
	certPool.AddCert(parsedCert)

	slog.Debug("Insecure development certificate generated",
		"subject", parsedCert.Subject.String(),
		"serial", parsedCert.SerialNumber.String(),
	)

	if isServer {
		slog.Debug("Configuring server-side TLS with optional Client Auth verification for dev mode")
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequestClientCert,
			ClientCAs:    certPool,
		}, nil
	}

	slog.Debug("Configuring client-side TLS with Root CA trust")
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certPool,
		InsecureSkipVerify: true,
	}, nil
}
