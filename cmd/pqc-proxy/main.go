package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log/slog"
	"math/big"
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

	logger.Init(cfg.Debug)

	if cfg.Mode != "client" && cfg.Mode != "server" {
		slog.Error("Invalid execution mode", "mode", cfg.Mode, "expected", "client|server")
		os.Exit(1)
	}

	if cfg.ListenAddr == "" || cfg.TargetAddr == "" {
		slog.Error("Missing required network parameters", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
		os.Exit(1)
	}

	slog.Info("Initializing pqc-proxy", "mode", cfg.Mode, "version", "1.1.0")

	pqcKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		slog.Error("Failed to initialize PQC keys", "error", err)
		os.Exit(1)
	}
	slog.Info("PQC keys generated successfully")

	metrics.Init()
	go func() {
		slog.Info("Starting Prometheus metrics server", "addr", cfg.MetricsAddr)
		if err := metrics.StartServer(cfg.MetricsAddr); err != nil {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"PQC-Proxy-Dev"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(&template)

	if isServer {
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		}, nil
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certPool,
		InsecureSkipVerify: true,
	}, nil
}
