
# pqc-proxy (Hybrid Post-Quantum TCP Tunnel)

A lightweight infrastructure proxy server (Client/Server architecture) engineered to secure legacy TCP application traffic against interception (sniffing) and retrospective decryption utilizing quantum cryptanalysis methodologies (e.g., Shor's algorithm).

## Architecture

The project implements a **Crypto-Agility** paradigm through a hybrid key exchange mechanism, combining classical elliptic curve cryptography with post-quantum lattice-based key encapsulation mechanisms.

* **Classical Layer:** Diffie-Hellman over Curve25519 (`X25519`) to maintain backwards compatibility and mitigate conventional cryptographic threats.
* **Post-Quantum Layer:** `ML-KEM-768` Key Encapsulation Mechanism (NIST FIPS 203 standard), providing quantum-safe security equivalent to AES-192.
* **Key Derivation Function (KDF):** `HKDF-SHA256`. Shared secrets from both mathematical domains are concatenated and derived into a single, indivisible master key. Compromise of either cryptographic primitive independently does not lead to session compromise.
* **Transport Layer (AEAD):** The network stream is encapsulated within a custom abstraction layer overriding `net.Conn`. Data is fragmented into discrete frames (up to 32 KB) and encrypted using the `ChaCha20-Poly1305` authenticated encryption cipher. A monotonically incremented nonce is enforced per frame to mitigate Replay Attacks.
* **Memory Management:** Socket-to-socket data relaying utilizes a `sync.Pool` buffer allocation system. Dynamic heap allocations are eliminated during the hot path data transfer phase (Zero-Allocation Pipeline), minimizing Latency and mitigating Garbage Collector overhead under peak network workloads.

## Project Structure

```text
├── .github/workflows/   # CI/CD pipelines (GitHub Actions)
├── cmd/
│   └── pqc-proxy/       # Application entry point (main.go)
├── deployments/         # Docker Compose configurations and Prometheus manifests
├── internal/
│   ├── config/          # Configuration parsing module
│   ├── crypto/          # Cryptographic core engine (X25519, ML-KEM, Conn)
│   ├── logger/          # Structured logging component
│   ├── metrics/         # Prometheus metrics exporter
│   └── network/         # Network engine (Client, Server, Buffer Pool)
├── scripts/             # Automation scripts for testing and benchmarking
└── web/                 # Static files for the monitoring dashboard

```

## Prerequisites

* Go 1.24 or higher
* Python 3.x (Optional, required for local verification environments)

## Compilation

Compile the binary from the root directory of the repository:

### Windows:

```bash
go build -o pqc-proxy.exe ./cmd/pqc-proxy

```

### Linux / macOS:

```bash
go build -o pqc-proxy ./cmd/pqc-proxy

```

## Verification & Testing

Execute the complete unit test suite validating the cryptographic core and network pipeline:

```bash
go test -v ./internal/...

```

### Environment Simulation Topology:

```text
[Legacy Client] -> (TCP:3000) -> [PQC Client] -> (PQC Tunnel:9090) -> [PQC Server] -> (TCP:8000) -> [Target App]

```

1. **Initialize the Target Application (Backend Service):**
```bash
python -m http.server 8000

```


2. **Execute the Proxy in SERVER Mode:**
```bash
./pqc-proxy -mode server -listen :9090 -target 127.0.0.1:8000

```


3. **Execute the Proxy in CLIENT Mode:**
```bash
./pqc-proxy -mode client -listen :3000 -target 127.0.0.1:9090

```


4. **Verify End-to-End Encrypted Proxying:**
```bash
curl [http://127.0.0.1:3000](http://127.0.0.1:3000)

```



## License

This project is distributed under the MIT License. Refer to the `LICENSE` file for full legal provisions.