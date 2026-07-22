package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"sync"
	"time"
)

type SessionState struct {
	ID        []byte
	MasterKey []byte
	CreatedAt time.Time
}

type TdkManager struct {
	mu          sync.RWMutex
	currentKey  []byte
	previousKey []byte
	interval    time.Duration
	stopChan    chan struct{}
}

func ZeroizeMem(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func NewTdkManager(rotationInterval time.Duration) (*TdkManager, error) {
	initialKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, initialKey); err != nil {
		return nil, err
	}

	mgr := &TdkManager{
		currentKey: initialKey,
		interval:   rotationInterval,
		stopChan:   make(chan struct{}),
	}

	if rotationInterval > 0 {
		go mgr.startAutoRotation()
	}

	return mgr, nil
}

func (m *TdkManager) RotateKeys() error {
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.previousKey != nil {
		ZeroizeMem(m.previousKey)
	}

	m.previousKey = m.currentKey
	m.currentKey = newKey

	return nil
}

func (m *TdkManager) startAutoRotation() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.RotateKeys()
		case <-m.stopChan:
			return
		}
	}
}

func (m *TdkManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.stopChan)

	if m.currentKey != nil {
		ZeroizeMem(m.currentKey)
		m.currentKey = nil
	}
	if m.previousKey != nil {
		ZeroizeMem(m.previousKey)
		m.previousKey = nil
	}
}

func (m *TdkManager) IssueTicket(state *SessionState) ([]byte, error) {
	m.mu.RLock()
	key := make([]byte, len(m.currentKey))
	copy(key, m.currentKey)
	m.mu.RUnlock()

	defer ZeroizeMem(key)

	return encryptWithKey(key, state)
}

func (m *TdkManager) ValidateTicket(ticket []byte) (state *SessionState, needsReissue bool, err error) {
	m.mu.RLock()
	currKey := make([]byte, len(m.currentKey))
	copy(currKey, m.currentKey)

	var prevKey []byte
	if m.previousKey != nil {
		prevKey = make([]byte, len(m.previousKey))
		copy(prevKey, m.previousKey)
	}
	m.mu.RUnlock()

	defer ZeroizeMem(currKey)
	if prevKey != nil {
		defer ZeroizeMem(prevKey)
	}

	state, err = decryptWithKey(currKey, ticket)
	if err == nil {
		return state, false, nil
	}

	if prevKey != nil {
		state, errPrev := decryptWithKey(prevKey, ticket)
		if errPrev == nil {
			return state, true, nil
		}
	}

	return nil, false, errors.New("failed to decrypt ticket with active keys")
}

func encryptWithKey(key []byte, state *SessionState) ([]byte, error) {
	block, err := aes.NewCipher(key)
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

	defer ZeroizeMem(plainText)

	return gcm.Seal(nonce, nonce, plainText, nil), nil
}

func decryptWithKey(key []byte, ticket []byte) (*SessionState, error) {
	block, err := aes.NewCipher(key)
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

	defer ZeroizeMem(plainText)

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
