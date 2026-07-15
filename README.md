# Latch (Hybrid Post-Quantum Tunnel)

A lightweight, high-performance infrastructure proxy server engineered to secure legacy TCP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

---

## What's New in 1.1.0 & 1.2.0

* **Session Resumption (Fast Reconnect):** Integrated a stateful, secure session-resumption layer (using `SessionStore` and AES-GCM Ticket encryption). Clients can now bypass heavy ML-KEM math on reconnect, establishing secure transport in a fraction of the time.
* **Mutual TLS (mTLS) Transport:** Replaced legacy symmetric pre-shared secrets with a robust, asymmetric Mutual TLS layer. Nodes are verified via an X.509 Public Key Infrastructure (PKI) prior to executing post-quantum handshakes.
* **Asynchronous Handshake Architecture:** Moved server-side TLS handshakes into a concurrent worker pool context to keep the primary `Accept` loop unblocked, completely eliminating socket read reset errors (`wsarecv`).
* **Cross-Platform Automation Pipeline:** Upgraded the testing suite. `test.ps1` (Windows) and `test.sh` (Linux/macOS) now automatically perform formatting, module tidying, linter checks, fuzzing, race detection, and cross-compile optimized binaries directly into `./dist/`.
* **Zero-Allocation Data Path:** Guaranteed raw performance (**0 B/op**, **0 allocs/op**) across the core `proxyPipe` routing layer via structured `sync.Pool` byte-buffer recycling.

---

## Security Design

`Latch` establishes a multi-layered security perimeter:

1. **Network Authentication (mTLS):** A standard TLS 1.3 handshake authenticates both endpoints. Unauthorized packets are dropped immediately, shielding expensive post-quantum mathematical calculations from unauthorized clients.
2. **Hybrid Post-Quantum Core:** Upon successful transport authorization, an ephemeral **X25519** (classical ECDH) and **ML-KEM-768** (NIST FIPS 203 quantum-resistant KEM) hybrid exchange executes. A 256-bit master key is derived via HKDF-SHA256.
3. **Payload Encryption:** Packets are encapsulated inside custom binary frames and encrypted using **ChaCha20-Poly1305** authenticated encryption (AEAD).

---

## System Topology

```text
[Client App] -> (Local:3000) -> [Latch Client] -> (mTLS + PQC Tunnel) -> [Latch Server] -> (Target:8000) -> [Backend App]

```

---

## Quick Start

### Start Server

```bash
./latch -mode server -listen :9090 -target 127.0.0.1:8000

```

### Start Client

```bash
./latch -mode client -listen :3000 -target 127.0.0.1:9090

```

> *Note: Valid client/server certificates and root CA bundles must be properly bound to your environment's cryptographic configurations.*

---

## Verification & Automation

Run the full pipeline (static code analysis, fuzzing, race detection, and localized artifact compilation for Windows, Linux, and macOS):

### On Windows (PowerShell):

```powershell
.\test.ps1

```

### On Linux & macOS (Bash):

```bash
chmod +x test.sh
./test.sh

```

### Benchmarking Routing Efficiency:

```bash
go test -run=^$ -bench=BenchmarkProxyPipe -benchmem ./internal/network/tests/...

```

### Independent Frame Parser Fuzzing:

```bash
go test -fuzz=FuzzSecureConnRead -fuzztime=10s ./internal/crypto

```

---

## Next-Gen Architecture: Fast-Path Rust Data Plane

To achieve maximum throughput, zero garbage collector overhead, and ultra-low latency under high packet rates (especially for UDP traffic), `Latch` is moving toward a **Control Plane / Data Plane separation** model.

### Split-Process Model (Go + Rust)

```text
                           +------------------------+
                           |   Latch Go-Control     |  <-- Config, mTLS,
                           |   (Management Plane)   |      PQC Handshake
                           +------------------------+
                                       |
                                       | Sends session key & remote details
                                       v (via Unix Socket / Named Pipe)
+----------------+         +------------------------+         +----------------+
|  Local Client  |  <--->  |  Latch Rust Dataplane  |  <--->  | Remote Server  |
|  (TCP / UDP)   |         | (High-Speed Data Path) |         |  (Encrypted)   |
+----------------+         +------------------------+         +----------------+
                             * Zero-Copy Ring Buffers
                             * Tokio Async Runtime
                             * AEAD Crypto in Rust

```

* **Control Plane (Go):** Handles the configuration, standard mTLS handshake, PQC hybrid key exchange, and session resumption management. Once a connection is authorized and keys are agreed upon, Go passes the active context down to the fast path.
* **Data Plane (Rust):** A dedicated, highly-optimized daemon processes the actual TCP/UDP streams. It receives the session keys over a local Unix domain socket (or Windows named pipe), opening high-speed, zero-copy sockets for raw packet forwarding and fast AEAD encryption.

---

## Roadmap

* [x] Hybrid Key Exchange (X25519 + ML-KEM-768)
* [x] Structured Production Logging via `log/slog`
* [x] Zero-Allocation Network Pipeline (`sync.Pool`)
* [x] Certificate-based Authentication / Mutual TLS (mTLS)
* [x] Automated Multi-OS Build Pipeline (`/dist` for Windows, Linux, macOS)
* [x] Native Go Fuzzing for custom frame parsing
* [x] Session Resumption (Fast Reconnect) for Hybrid Handshakes
* [ ] **Next:** Split-Process Fast-Path Data Plane (Rust acceleration)
* [ ] UDP Encapsulation / Tunneling Mode
* [ ] Transparent proxying (TPROXY) and iptables redirection support