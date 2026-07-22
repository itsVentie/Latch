package crypto_test

import (
	"bytes"
	"testing"
	"time"

	"pqc-proxy/internal/crypto"
)

func TestTdkManager_LifecycleAndRotation(t *testing.T) {
	mgr, err := crypto.NewTdkManager(0)
	if err != nil {
		t.Fatalf("failed to init TDK manager: %v", err)
	}
	defer mgr.Stop()

	state := &crypto.SessionState{
		ID:        bytes.Repeat([]byte{0x01}, 32),
		MasterKey: bytes.Repeat([]byte{0xAA}, 32),
		CreatedAt: time.Now(),
	}

	ticket1, err := mgr.IssueTicket(state)
	if err != nil {
		t.Fatalf("failed to issue ticket: %v", err)
	}

	decState, needsReissue, err := mgr.ValidateTicket(ticket1)
	if err != nil {
		t.Fatalf("failed to validate ticket: %v", err)
	}
	if needsReissue {
		t.Errorf("expected needsReissue=false for active key, got true")
	}
	if !bytes.Equal(decState.MasterKey, state.MasterKey) {
		t.Errorf("master key mismatch: got %x, want %x", decState.MasterKey, state.MasterKey)
	}

	if err := mgr.RotateKeys(); err != nil {
		t.Fatalf("failed to rotate keys: %v", err)
	}

	decState2, needsReissue2, err := mgr.ValidateTicket(ticket1)
	if err != nil {
		t.Fatalf("failed to validate ticket with previous key: %v", err)
	}
	if !needsReissue2 {
		t.Errorf("expected needsReissue=true after rotation, got false")
	}
	if !bytes.Equal(decState2.MasterKey, state.MasterKey) {
		t.Errorf("master key mismatch after rotation")
	}

	ticket2, err := mgr.IssueTicket(state)
	if err != nil {
		t.Fatalf("failed to issue ticket on rotated key: %v", err)
	}

	if err := mgr.RotateKeys(); err != nil {
		t.Fatalf("failed second rotation: %v", err)
	}

	_, _, err = mgr.ValidateTicket(ticket1)
	if err == nil {
		t.Errorf("expected error for ticket encrypted with dropped key, got nil")
	}

	_, needsReissue3, err := mgr.ValidateTicket(ticket2)
	if err != nil || !needsReissue3 {
		t.Errorf("expected ticket2 to be valid with needsReissue=true, got err: %v, reissue: %v", err, needsReissue3)
	}
}

func TestZeroizeMem(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	crypto.ZeroizeMem(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("byte at index %d is not zeroized: got %x", i, b)
		}
	}
}
