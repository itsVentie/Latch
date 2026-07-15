package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"time"
)

type SessionState struct {
	ID        []byte
	MasterKey []byte
	CreatedAt time.Time
}

type TicketEncryptor struct {
	key []byte
}

func NewTicketEncryptor() (*TicketEncryptor, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return &TicketEncryptor{key: key}, nil
}

func (te *TicketEncryptor) Encrypt(state *SessionState) ([]byte, error) {
	block, err := aes.NewCipher(te.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	plainText := make([]byte, len(state.ID)+len(state.MasterKey))
	copy(plainText[0:32], state.ID)
	copy(plainText[32:], state.MasterKey)

	cipherText := gcm.Seal(nonce, nonce, plainText, nil)
	return cipherText, nil
}

func (te *TicketEncryptor) Decrypt(ticket []byte) (*SessionState, error) {
	block, err := aes.NewCipher(te.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	limit := gcm.NonceSize()
	if len(ticket) < limit {
		return nil, errors.New("invalid ticket size")
	}

	nonce, encrypted := ticket[:limit], ticket[limit:]
	plainText, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, err
	}

	if len(plainText) < 64 {
		return nil, errors.New("decrypted data too short")
	}

	state := &SessionState{
		ID:        make([]byte, 32),
		MasterKey: make([]byte, 32),
		CreatedAt: time.Now(),
	}
	copy(state.ID, plainText[0:32])
	copy(state.MasterKey, plainText[32:64])

	return state, nil
}
