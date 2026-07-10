# pqc-proxy (Hybrid Post-Quantum TCP Tunnel)

A lightweight infrastructure proxy server engineered to secure legacy TCP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

## What's New in 1.1.0
* **Mutual TLS (mTLS) Transport:** Replaced the legacy symmetric pre-shared secret verification (`-secret`) with robust, asymmetric Mutual TLS. Nodes are now verified via a Public Key Infrastructure (PKI) layer prior to the post-quantum exchange.
* **Asynchronous Handshake Architecture:** Moved the server-side TLS handshake into a concurrent worker pool context. This unblocks the primary `Accept` loop, preventing deadlocks and eliminating socket read reset errors (`wsarecv`).
* **Automated Multi-Platform Pipeline:** Enhanced the `test.ps1` script to automatically cross-compile optimized production binaries for both Windows (`amd64`) and Linux (`amd64`), isolating them inside the newly managed `./dist/` directory.
* **Zero-Allocation Data Path:** Maintained strict zero-pressure memory recycling (**0 B/op**, **0 allocs/op**) across the core `proxyPipe` data routing layer via structured `sync.Pool` byte-buffer reuse.

## Why Post-Quantum?
Traditional asymmetric cryptography (RSA, ECDH) is fundamentally vulnerable to future quantum computing scaling. `pqc-proxy` establishes a dual-defense security perimeter:
1. **Transport Layer (mTLS):** X509 certificate-based mutual authentication.
2. **Post-Quantum Layer:** Hybrid Key Encapsulation Mechanism combining classical **X25519** (ECDH) and quantum-resistant **ML-KEM-768** (NIST FIPS 203).

## Quick Start Topology
```text
[Client App] -> (Local:3000) -> [PQC Client] -> (mTLS + PQC Tunnel) -> [PQC Server] -> (Target:8000) -> [Backend App]

```

### Start Server

```bash
./pqc-proxy -mode server -listen :9090 -target 127.0.0.1:8000

```

### Start Client

```bash
./pqc-proxy -mode client -listen :3000 -target 127.0.0.1:9090

```

*> Note: Ensure that valid client/server certificates and root CA bundles are properly bound to the respective cryptographic configurations.*

## Verification & Automation

The project includes a comprehensive verification workflow via PowerShell to perform static code analysis, race detection, and localized artifact compilation:

```powershell
# Run formatter, linter, race detector, and compile binaries to /dist
.\test.ps1

```

To profile execution times, packet routing efficiency, and memory allocations under simulated high load:

```bash
go test -run=^$ -bench=BenchmarkProxyPipe -benchmem ./internal/network/tests/...

```

## Security Design

The communication sequence utilizes a modern defense-in-depth model:

* **Network Authentication:** TLS 1.3 mTLS handshake authenticates both nodes. Packets failing standard TLS validation are dropped at the packet boundary before executing expensive post-quantum mathematical calculations.
* **Hybrid Core:** Upon successful transport authorization, a non-separable ephemeral X25519 and ML-KEM-768 key exchange executes, deriving a 256-bit symmetric master key via HKDF-SHA256.
* **Payload Encryption:** Traffic is encapsulated inside custom authenticated frames and encrypted using **ChaCha20-Poly1305** AEAD.

## Roadmap

* [x] Hybrid Key Exchange (X25519 + ML-KEM-768)
* [x] Structured Production Logging via `log/slog`
* [x] Zero-Allocation Network Pipeline (`sync.Pool`)
* [x] Certificate-based Authentication / Mutual TLS (mTLS)
* [x] Automated Automated Multi-OS Build Pipeline (`/dist`)
* [ ] Session Resumption (Fast Reconnect) for Hybrid Handshakes
* [ ] UDP Encapsulation / Tunneling Mode
