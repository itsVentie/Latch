# pqc-proxy (Hybrid Post-Quantum TCP Tunnel)

A lightweight infrastructure proxy server engineered to secure legacy TCP traffic against interception and future quantum cryptanalysis (Shor's algorithm).

## What's New in 1.0.1

* **Secure Handshake Authentication:** Implemented HMAC-based authentication to prevent unauthorized connection attempts.
* **Automation Suite:** Added `test.ps1` for comprehensive validation (vet, race detection, and build).
* **Improved Stability:** Enhanced internal error handling for port binding and process lifecycle.

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

We provide an automated script to ensure the integrity of the codebase, including static analysis and race detection:

**Windows:**

```powershell
.\test.ps1

```

## Security Design

The system utilizes a two-stage verification process:

1. **Auth Layer:** HMAC-SHA256 token exchange (using a pre-shared secret). Unauthorized packets are dropped before initiating heavy PQC key exchanges.
2. **Encryption Layer:** Hybrid KEM exchange followed by `ChaCha20-Poly1305` transport encryption.

## Roadmap

* [x] Hybrid Key Exchange (X25519 + ML-KEM-768)
* [x] HMAC-based Connection Auth
* [x] CI/CD Pipeline & Auto-testing
* [ ] UDP Support
* [ ] Certificate-based Authentication
