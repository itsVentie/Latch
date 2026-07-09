package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	slog.Info("Initializing pqc-proxy", "mode", cfg.Mode, "version", "0.1.4")

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

	if cfg.Mode == "server" {
		srv := network.NewServer(cfg.ListenAddr, cfg.TargetAddr, pqcKeys, cfg.Secret)
		slog.Info("Starting PQC SERVER", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
		go func() {
			if err := srv.Start(); err != nil {
				slog.Error("Server runtime error", "error", err)
				os.Exit(1)
			}
		}()
		<-sigChan
		srv.Stop()
	} else {
		cli := network.NewClient(cfg.ListenAddr, cfg.TargetAddr, pqcKeys, cfg.Secret)
		slog.Info("Starting PQC CLIENT", "listen", cfg.ListenAddr, "target", cfg.TargetAddr)
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
