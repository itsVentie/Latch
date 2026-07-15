package network

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"

	"pqc-proxy/internal/crypto"
)

type SessionStore struct {
	sessions sync.Map
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	store := &SessionStore{ttl: ttl}
	go store.cleanupLoop()
	return store
}

func (s *SessionStore) GenerateSessionID() ([]byte, error) {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return nil, err
	}
	return id, nil
}

func (s *SessionStore) Put(id []byte, masterKey []byte) {
	key := hex.EncodeToString(id)
	state := &crypto.SessionState{
		ID:        id,
		MasterKey: masterKey,
		CreatedAt: time.Now(),
	}
	s.sessions.Store(key, state)
}

func (s *SessionStore) Get(id []byte) ([]byte, bool) {
	key := hex.EncodeToString(id)
	val, ok := s.sessions.Load(key)
	if !ok {
		return nil, false
	}

	state := val.(*crypto.SessionState)
	if time.Since(state.CreatedAt) > s.ttl {
		s.sessions.Delete(key)
		return nil, false
	}

	return state.MasterKey, true
}

func (s *SessionStore) Delete(id []byte) {
	s.sessions.Delete(hex.EncodeToString(id))
}

func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(s.ttl / 2)
	for range ticker.C {
		s.sessions.Range(func(key, val interface{}) bool {
			state := val.(*crypto.SessionState)
			if time.Since(state.CreatedAt) > s.ttl {
				s.sessions.Delete(key)
			}
			return true
		})
	}
}
