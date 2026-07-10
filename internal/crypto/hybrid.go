package crypto

import (
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	X25519PublicKeySize    = 32
	MLKEM768PublicKeySize  = 1184
	MLKEM768CiphertextSize = 1088
	ClientInceptionSize    = X25519PublicKeySize + MLKEM768PublicKeySize
	ServerResponseSize     = X25519PublicKeySize + MLKEM768CiphertextSize
)

type KeyExchangeResult struct {
	SharedMasterKey []byte
	PublicKeyBlob   []byte
}

func GenerateClientInception() (*ecdh.PrivateKey, *mlkem.DecapsulationKey768, []byte, error) {
	ecdhPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate X25519 key: %w", err)
	}
	ecdhPub := ecdhPriv.PublicKey().Bytes()

	mlkemPriv, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ML-KEM key: %w", err)
	}
	mlkemPub := mlkemPriv.EncapsulationKey().Bytes()

	blob := make([]byte, 0, ClientInceptionSize)
	blob = append(blob, ecdhPub...)
	blob = append(blob, mlkemPub...)

	return ecdhPriv, mlkemPriv, blob, nil
}

func ServerHandleInception(clientBlob []byte) ([]byte, []byte, error) {
	if len(clientBlob) < ClientInceptionSize {
		return nil, nil, fmt.Errorf("invalid client blob size")
	}

	clientEcdhPubBytes := clientBlob[:X25519PublicKeySize]
	clientMlkemPubBytes := clientBlob[X25519PublicKeySize:ClientInceptionSize]

	serverEcdhPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("server ecdh generation failed: %w", err)
	}

	clientEcdhPub, err := ecdh.X25519().NewPublicKey(clientEcdhPubBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid client ecdh public key: %w", err)
	}

	ecdhSecret, err := serverEcdhPriv.ECDH(clientEcdhPub)
	if err != nil {
		return nil, nil, fmt.Errorf("ecdh computation failed: %w", err)
	}

	clientMlkemPub, err := mlkem.NewEncapsulationKey768(clientMlkemPubBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid client mlkem public key: %w", err)
	}

	mlkemSharedSecret, mlkemCiphertext := clientMlkemPub.Encapsulate()

	masterKey, err := deriveHybridMasterKey(ecdhSecret, mlkemSharedSecret)
	if err != nil {
		return nil, nil, err
	}

	serverEcdhPubBytes := serverEcdhPriv.PublicKey().Bytes()
	responseBlob := make([]byte, 0, ServerResponseSize)
	responseBlob = append(responseBlob, serverEcdhPubBytes...)
	responseBlob = append(responseBlob, mlkemCiphertext...)

	return masterKey, responseBlob, nil
}

func ClientHandleResponse(ecdhPriv *ecdh.PrivateKey, mlkemPriv *mlkem.DecapsulationKey768, serverBlob []byte) ([]byte, error) {
	if len(serverBlob) < ServerResponseSize {
		return nil, fmt.Errorf("invalid server blob size")
	}

	serverEcdhPubBytes := serverBlob[:X25519PublicKeySize]
	serverMlkemCiphertext := serverBlob[X25519PublicKeySize:ServerResponseSize]

	serverEcdhPub, err := ecdh.X25519().NewPublicKey(serverEcdhPubBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid server ecdh public key: %w", err)
	}

	ecdhSecret, err := ecdhPriv.ECDH(serverEcdhPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh computation failed: %w", err)
	}

	mlkemSharedSecret, err := mlkemPriv.Decapsulate(serverMlkemCiphertext)
	if err != nil {
		return nil, fmt.Errorf("mlkem decapsulation failed: %w", err)
	}

	return deriveHybridMasterKey(ecdhSecret, mlkemSharedSecret)
}

func deriveHybridMasterKey(ecdhSecret, mlkemSecret []byte) ([]byte, error) {
	combinedSecret := make([]byte, 0, len(ecdhSecret)+len(mlkemSecret))
	combinedSecret = append(combinedSecret, ecdhSecret...)
	combinedSecret = append(combinedSecret, mlkemSecret...)

	kdf := hkdf.New(sha256.New, combinedSecret, nil, []byte("PQC-PROXY-HYBRID-KEY-v1"))
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(kdf, masterKey); err != nil {
		return nil, fmt.Errorf("hkdf extraction failed: %w", err)
	}

	return masterKey, nil
}
