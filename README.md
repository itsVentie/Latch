# Latch (Hybrid Post-Quantum Tunnel)

A lightweight, high-performance infrastructure proxy server engineered to secure legacy TCP and UDP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

---

## What's New

* **Automatic TDK Key Rotation & Ticket Reissuance:** Integrated a stateful `TdkManager` for managing Session Ticket Encryption Keys (TDK). Keys automatically rotate based on configurable TTLs, while maintaining smooth fallback for active sessions and triggering automatic ticket updates for re-connecting clients.
* **Secure Memory Wiping (`ZeroizeMem`):** Hardened runtime security by implementing explicit zeroization of sensitive cryptographic buffers, master keys, and private KEM material immediately after usage.
* **Production Split-Process Architecture:** Fast-path architecture separating the **Go Control Plane** (management, routing, and key agreement) from the native **Rust Data Plane** (high-speed packet processing).
* **End-to-End UDP Encapsulation (Issue #5 Resolved):** Full tunneling support for UDP traffic. Packets are captured by the proxy, wrapped inside custom quantum-resistant secure frames, encrypted using ChaCha20-Poly1305, and seamlessly decapsulated at the target node.
* **Deterministic Windows Port Handover:** Re-engineered the control plane socket lifecycle to handle Windows-specific port locking smoothly. Fixed all `WSAEADDRINUSE` collisions and integration test race conditions during Go-to-Rust delegation context transfers.

---

## Security Design

`Latch` establishes a multi-layered security perimeter:

1. **Network Authentication (mTLS 1.3):** A standard TLS 1.3 handshake authenticates both endpoints using certificates. Unauthorized packets are dropped immediately, shielding expensive post-quantum mathematical calculations from unauthorized clients.
2. **Hybrid Post-Quantum Core:** Upon successful transport authorization, an ephemeral **X25519** (classical ECDH) and **ML-KEM-768** (NIST FIPS 203 quantum-resistant KEM) hybrid exchange executes. A 256-bit master key is derived via HKDF-SHA256.
3. **Stateful Session Resumption & TDK Rotation:** Reuses negotiated master keys via encrypted session tickets. Tickets are secured using periodically rotated TDK keys, preventing long-term session ticket decryption if a single key is compromised.
4. **Payload Encryption & Zeroize:** Packets are encapsulated inside custom binary frames and encrypted using **ChaCha20-Poly1305** authenticated encryption (AEAD). Sensitive cryptographic material is immediately zeroed in memory (`ZeroizeMem`) upon handshake completion.

---

## Production Architecture: Split-Process Fast-Path

Latch leverages a split-process design to achieve maximum throughput, zero garbage collector overhead, and ultra-low latency under heavy high-frequency packet rates.

```text
                      +------------------------+
                      |   Latch Go-Control     |  <-- Configuration, mTLS,
                      |   (Management Plane)   |      PQC Hybrid Handshake,
                      +------------------------+      TDK Rotation & SessionStore
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

* **Control Plane (Go):** Manages daemon orchestration, mutual TLS (mTLS 1.3) infrastructure, stateful session resumption, TDK rotation, and the core post-quantum hybrid key exchange (X25519 + ML-KEM-768). Once authenticated, Go safely delegates session keys down to the packet engine.
* **Data Plane (Rust):** A highly optimized, asynchronous native daemon (`latch-dataplane`) that intercepts and processes raw TCP and UDP streams. Built on top of the `tokio` runtime, it bypasses Go's runtime scheduling and GC pauses for heavy packet processing.

---

## System Topology

```text
[Client App] -> (Local:3000) -> [Latch Client] -> (mTLS + PQC Tunnel) -> [Latch Server] -> (Target:8000) -> [Backend App]

```

---

## Configuration & CLI Options

### Flags Reference

| Flag | Default | Description |
| --- | --- | --- |
| `--mode` | `server` | Operating mode (`server` or `client`) |
| `--listen` | `127.0.0.1:4000` | Address and port to bind the proxy listener |
| `--target` | `127.0.0.1:8080` | Destination address to forward decrypted traffic |
| `--secret` | *required* | Pre-shared master secret string for initial authentication |
| `--cert` | `certs/server.crt` | Path to TLS certificate file (mTLS) |
| `--key` | `certs/server.key` | Path to TLS private key file (mTLS) |
| `--ca` | `certs/ca.crt` | Path to Root CA bundle for peer verification |
| `--tdk-ttl` | `24h` | Time-to-live for a Session Ticket Encryption Key |
| `--tdk-rotation` | `1h` | Frequency interval for automated TDK key rotation |
| `--debug` | `false` | Enable verbose structured debug logs (`slog` JSON/Text) |
| `--pprof` | `false` | Enable runtime profiling server on `127.0.0.1:6060` |

---

## Quick Start

### 1. Start Server (Go Control Plane + Rust Dataplane)

```bash
./dist/latch.exe \
  --mode server \
  --listen 127.0.0.1:4000 \
  --target 127.0.0.1:8080 \
  --secret "secret-key" \
  --cert certs/server.crt \
  --key certs/server.key \
  --ca certs/ca.crt \
  --tdk-ttl 12h \
  --tdk-rotation 30m \
  --debug

```

### 2. Start Client

```bash
./dist/latch.exe \
  --mode client \
  --listen 127.0.0.1:3000 \
  --target 127.0.0.1:4000 \
  --secret "secret-key" \
  --cert certs/client.crt \
  --key certs/client.key \
  --ca certs/ca.crt \
  --debug

```

---

## Testing & Verification Suite

### Automated Multi-Platform Pipeline

Run complete validation (formatting, static code analysis, native fuzzing, race detection, and localized cross-compilation for Windows, Linux, and macOS):

**On Windows (PowerShell):**

```powershell
.\test.ps1

```

**On Linux & macOS (Bash):**

```bash
chmod +x test.sh
./test.sh

```

### Granular Testing Commands

#### 1. Unit & Integration Tests (With Race Detector)

```bash
# Run all crypto & network unit tests with CGO race detection
CGO_ENABLED=1 go test -v -race ./...

# Run TDK Key Rotation & Memory Zeroization tests specifically
go test -v -run "TestTdkManager|TestZeroizeMem" ./internal/crypto/...

# Run Session Resumption & E2E Proxy integration tests
go test -v -run "TestSessionResumptionFlow|TestEndToEndProxy" ./internal/network/tests/...

```

#### 2. Native Go Fuzzing (Custom Frame Parser)

```bash
# Fast 10-second parser fuzzing execution
go test -fuzz=FuzzSecureConnRead -fuzztime=10s ./internal/crypto

# Extended fuzzing session for stress testing
go test -fuzz=FuzzSecureConnRead -fuzztime=5m ./internal/crypto

```

#### 3. Performance & Memory Allocation Benchmarks

```bash
# Measure zero-allocation pipeline and proxy pipe throughput
go test -run=^$ -bench=BenchmarkProxyPipe -benchmem ./internal/network/tests/...

# Benchmark Hybrid Key Exchange (X25519 + ML-KEM-768) math
go test -run=^$ -bench=BenchmarkHybridHandshake -benchmem ./internal/crypto/...

```

#### 4. Profiling & Memory Leak Audit (pprof)

Start `latch` with the `--pprof` flag, then capture heap/goroutine profiles:

```bash
# Capture heap profile under load
go tool pprof [http://127.0.0.1:6060/debug/pprof/heap](http://127.0.0.1:6060/debug/pprof/heap)

# Inspect memory allocations in interactive mode
(pprof) top20 -cum

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
* [x] Automatic TDK Key Rotation & Memory Zeroization (`ZeroizeMem`)
* [x] Split-Process Fast-Path Data Plane (Rust acceleration engine)
* [x] UDP Encapsulation / Tunneling Mode (Issue #5 closed)
* [ ] Transparent proxying (TPROXY) and iptables redirection support
* [ ] Native File Descriptor passing (`SCM_RIGHTS` / `DuplicateHandle`) via IPC


