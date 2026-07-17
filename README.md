# Latch (Hybrid Post-Quantum Tunnel)

A lightweight, high-performance infrastructure proxy server engineered to secure legacy TCP and UDP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

---

## What's New in 1.3.0

* **Production Split-Process Architecture:** Officially migrated to a fast-path architecture separating the **Go Control Plane** (management, routing, and key agreement) from the native **Rust Data Plane** (high-speed packet processing).
* **End-to-End UDP Encapsulation (Issue #5 Resolved):** Full tunneling support for UDP traffic. Packets are captured by the proxy, wrapped inside custom quantum-resistant secure frames, encrypted using ChaCha20-Poly1305, and seamlessly decapsulated at the target node.
* **Deterministic Windows Port Handover:** Re-engineered the control plane socket lifecycle to handle Windows-specific port locking smoothly. Fixed all `WSAEADDRINUSE` collisions and integration test race conditions during Go-to-Rust delegation context transfers.
* **Session Resumption (Fast Reconnect):** Reuses negotiated hybrid master keys via stateful server-side `SessionStore` ticket management, bypassing expensive ML-KEM math on reconnections.

---

## Security Design

`Latch` establishes a multi-layered security perimeter:

1. **Network Authentication (mTLS):** A standard TLS 1.3 handshake authenticates both endpoints. Unauthorized packets are dropped immediately, shielding expensive post-quantum mathematical calculations from unauthorized clients.
2. **Hybrid Post-Quantum Core:** Upon successful transport authorization, an ephemeral **X25519** (classical ECDH) and **ML-KEM-768** (NIST FIPS 203 quantum-resistant KEM) hybrid exchange executes. A 256-bit master key is derived via HKDF-SHA256.
3. **Payload Encryption:** Packets are encapsulated inside custom binary frames and encrypted using **ChaCha20-Poly1305** authenticated encryption (AEAD).

---

## Production Architecture: Split-Process Fast-Path

Latch leverages a split-process design to achieve maximum throughput, zero garbage collector overhead, and ultra-low latency under heavy high-frequency packet rates.

```text
                      +------------------------+
                      |   Latch Go-Control     |  <-- Configuration, mTLS,
                      |   (Management Plane)   |      PQC Hybrid Handshake
                      +------------------------+
                                  |
                                  | Secures transport context & generates master key
                                  v (Delegation via IPC / Named Pipes)
+----------------+    +------------------------+    +----------------+
|  Local Client  | <->|  Latch Rust Dataplane  | <->| Remote Server  |
|  (TCP / UDP)   |    | (High-Speed Data Path) |    |   (Encrypted)  |
+----------------+    +------------------------+    +----------------+
                       * Zero-Copy Ring Buffers
                       * Tokio Async Runtime
                       * Blazing fast ChaCha20-Poly1305 AEAD

```

* **Control Plane (Go):** Manages the daemon orchestration, mutual TLS (mTLS 1.3) infrastructure, stateful session resumption, and the core post-quantum hybrid key exchange (X25519 + ML-KEM-768). Once authenticated, Go safely delegates the session keys down to the packet engine.
* **Data Plane (Rust):** A highly optimized, asynchronous native daemon (`latch-dataplane`) that intercepts and processes raw TCP and UDP streams. Built on top of the `tokio` runtime, it bypasses Go's runtime scheduling and GC pauses for heavy packet processing.

---

## System Topology

```text
[Client App] -> (Local:3000) -> [Latch Client] -> (mTLS + PQC Tunnel) -> [Latch Server] -> (Target:8000) -> [Backend App]

```

---

## Quick Start

The binary automatically routes traffic depending on whether it intercepts TCP connections or UDP datagrams.

### Start Server

```bash
./dist/latch.exe --mode server --listen 127.0.0.1:4000 --target 127.0.0.1:8080 --secret "your-shared-secret" --debug

```

### Start Client

```bash
./dist/latch.exe --mode client --listen 127.0.0.1:3000 --target 127.0.0.1:4000 --secret "your-shared-secret" --debug

```

> *Note: Valid client/server certificates and root CA bundles must be properly bound to your environment's cryptographic configurations.*

---

## Verification & Automation

Run the complete pipeline (formatting, static code analysis, native fuzzing, race detection, and localized cross-compilation for Windows, Linux, and macOS):

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

## Roadmap

* [x] Hybrid Key Exchange (X25519 + ML-KEM-768)
* [x] Structured Production Logging via `log/slog`
* [x] Zero-Allocation Network Pipeline (`sync.Pool` integration)
* [x] Certificate-based Authentication / Mutual TLS (mTLS)
* [x] Automated Multi-OS Build Pipeline (`/dist` for Windows, Linux, macOS)
* [x] Native Go Fuzzing for custom frame parsing (`FuzzSecureConnRead`)
* [x] Session Resumption (Fast Reconnect) for Hybrid Handshakes
* [x] Split-Process Fast-Path Data Plane (Rust acceleration engine)
* [x] UDP Encapsulation / Tunneling Mode (Issue #5 closed)
* [ ] Transparent proxying (TPROXY) and iptables redirection support
* [ ] Native File Descriptor passing (`SCM_RIGHTS` / `DuplicateHandle`) via IPC
