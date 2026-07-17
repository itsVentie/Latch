package network_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"pqc-proxy/internal/crypto"
	"pqc-proxy/internal/network"
)

func TestEndToEndProxy(t *testing.T) {
	pqcKeys, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate PQC keys for test: %v", err)
	}

	targetAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8000")
	if err != nil {
		t.Fatalf("Failed to resolve target UDP address: %v", err)
	}

	target, err := net.ListenUDP("udp", targetAddr)
	if err != nil {
		t.Fatalf("Failed to bind target UDP: %v", err)
	}
	defer target.Close()

	go func() {
		buf := make([]byte, 2048)
		for {
			n, remoteAddr, err := target.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = target.WriteToUDP(buf[:n], remoteAddr)
		}
	}()

	srvTLS, cliTLS, err := generateTestmTLSPair()
	if err != nil {
		t.Fatalf("Failed to generate mTLS configs: %v", err)
	}

	srv := network.NewServer("127.0.0.1:9090", "127.0.0.1:8000", pqcKeys, srvTLS)
	go func() {
		_ = srv.Start()
	}()
	defer srv.Stop()

	cli := network.NewClient("127.0.0.1:3000", "127.0.0.1:9090", pqcKeys, cliTLS)
	go func() {
		_ = cli.Start()
	}()
	defer cli.Stop()

	time.Sleep(200 * time.Millisecond)

	proxyUDPAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:3000")
	if err != nil {
		t.Fatalf("Failed to resolve proxy UDP address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, proxyUDPAddr)
	if err != nil {
		t.Fatalf("Could not connect to client proxy via UDP: %v", err)
	}
	defer conn.Close()

	msg := []byte("quantum-safe-data")

	_, _ = conn.Write([]byte("trigger"))

	var n int
	buf := make([]byte, len(msg)+100)

	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)

		if _, err := conn.Write(msg); err != nil {
			continue
		}

		_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		n, err = conn.Read(buf)
		if err == nil {
			break
		}
	}

	if err != nil {
		t.Fatalf("Failed to read from proxy after retries: %v", err)
	}

	if string(buf[:n]) != string(msg) {
		t.Errorf("Expected %s, got %s", msg, buf[:n])
	}

	time.Sleep(50 * time.Millisecond)
}

func generateTestmTLSPair() (*tls.Config, *tls.Config, error) {
	caPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"PQC-Proxy-Test-CA"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caDer, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, err
	}

	caCert, err := x509.ParseCertificate(caDer)
	if err != nil {
		return nil, nil, err
	}

	srvPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	srvTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"PQC-Proxy-Test-Server"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	srvDer, err := x509.CreateCertificate(rand.Reader, &srvTemplate, caCert, &srvPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, err
	}

	srvCert := tls.Certificate{
		Certificate: [][]byte{srvDer},
		PrivateKey:  srvPriv,
	}

	cliPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	cliTemplate := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"PQC-Proxy-Test-Client"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	cliDer, err := x509.CreateCertificate(rand.Reader, &cliTemplate, caCert, &cliPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, err
	}

	cliCert := tls.Certificate{
		Certificate: [][]byte{cliDer},
		PrivateKey:  cliPriv,
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(caCert)

	srvTLS := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	cliTLS := &tls.Config{
		Certificates:       []tls.Certificate{cliCert},
		RootCAs:            certPool,
		InsecureSkipVerify: false,
		ServerName:         "localhost",
	}

	return srvTLS, cliTLS, nil
}
