# pqc-proxy (Hybrid Post-Quantum TCP Tunnel)

A lightweight infrastructure proxy server engineered to secure legacy TCP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

## What's New in 1.0.2

* **Structured Production Logging:** Fully migrated from standard text logs to Go's native `log/slog` library. Supports structured attributes (e.g., `remote_addr`, `local_addr`) and configurable log levels (`Debug`, `Info`, `Warn`, `Error`).
* **High-Load Performance Benchmarks:** Added network pipeline micro-benchmarking (`BenchmarkProxyPipe`) to validate data transfer efficiency under heavy load.
* **Zero-Allocation Core:** Verified `0 B/op` and `0 allocs/op` on the critical data path due to optimized `sync.Pool` buffer recycling, ensuring no garbage collection overhead during peak traffic.
* **Stabilized Integration Tests:** Resolved race conditions and fixed port lifecycle management within the end-to-end automated testing environment.

## Why Post-Quantum?

Traditional cryptography (RSA, ECDH) is vulnerable to future quantum computers. **pqc-proxy** implements a hybrid security layer:

* **Classical Layer:** `X25519` (Diffie-Hellman).
* **Quantum-Resistant Layer:** `ML-KEM-768` (NIST FIPS 203).
* **Auth Layer:** HMAC-SHA256 token verification.

## Quick Start Topology

```text
[Client App] -> (Local:3000) -> [PQC Client] -> (Encrypted Tunnel) -> [PQC Server] -> (Target:8000) -> [Backend App]

```

1. **Start Server:**

```bash
./pqc-proxy -mode server -listen :9090 -target 127.0.0.1:8000 -secret "YourStrongSecret"

```

2. **Start Client:**

```bash
./pqc-proxy -mode client -listen :3000 -target 127.0.0.1:9090 -secret "YourStrongSecret"

```

## Verification & Automation

We provide automated tools to ensure the integrity of the codebase, check performance metrics, and audit memory allocations:

**Run Formatting, Linter, and Race Detector:**

```powershell
.\test.ps1

```

**Run Micro-benchmarks and Allocation Audit:**

```bash
go test -run=^$ -bench=BenchmarkProxyPipe -benchmem ./internal/network/tests/...

```

## Security Design

The system utilizes a two-stage verification process:

1. **Auth Layer:** HMAC-SHA256 token exchange (using a pre-shared secret). Unauthorized packets are dropped before initiating heavy PQC key exchanges.
2. **Encryption Layer:** Hybrid KEM exchange followed by `ChaCha20-Poly1305` transport encryption.

## Roadmap

* [x] Hybrid Key Exchange (X25519 + ML-KEM-768)
* [x] HMAC-based Connection Auth
* [x] CI/CD Pipeline & Auto-testing
* [x] Structured JSON/Text Logging via `log/slog`
* [x] High-Load Benchmarking and Memory Profiling (Zero-Allocation verified)
* [ ] Certificate-based Authentication / Mutual TLS (mTLS)
* [ ] Session Resumption (Fast Reconnect) to Hybrid Handshake
* [ ] UDP Encapsulation / Tunneling
