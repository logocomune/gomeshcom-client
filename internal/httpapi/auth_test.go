package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestSessionStoreEvictExpiredRemovesOnlyStaleTokens(t *testing.T) {
	s := newSessionStore()

	validToken, _, err := s.create(time.Hour)
	if err != nil {
		t.Fatalf("create valid token: %v", err)
	}

	s.mu.Lock()
	s.sessions["stale-token"] = time.Now().UTC().Add(-time.Second)
	s.mu.Unlock()

	s.evictExpired()

	s.mu.Lock()
	_, stalePresent := s.sessions["stale-token"]
	_, validPresent := s.sessions[validToken]
	s.mu.Unlock()

	if stalePresent {
		t.Error("evictExpired: stale token still present after eviction")
	}
	if !validPresent {
		t.Error("evictExpired: valid token incorrectly removed")
	}
}

func TestSessionStoreEvictExpiredClearsAllExpired(t *testing.T) {
	s := newSessionStore()

	for range 5 {
		if _, _, err := s.create(time.Millisecond); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	time.Sleep(10 * time.Millisecond)
	s.evictExpired()

	s.mu.Lock()
	remaining := len(s.sessions)
	s.mu.Unlock()

	if remaining != 0 {
		t.Errorf("sessions after full eviction = %d, want 0", remaining)
	}
}

func TestSessionStoreStartStopsOnContextCancel(t *testing.T) {
	s := newSessionStore()
	ctx, cancel := context.WithCancel(context.Background())
	s.start(ctx)
	cancel()
	// Goroutine must drain and exit; no deadlock or panic.
	time.Sleep(20 * time.Millisecond)
}

func TestSessionStoreEvictExpiredDoesNotRemoveJustCreated(t *testing.T) {
	s := newSessionStore()

	token, _, err := s.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	s.evictExpired()

	if !s.valid(token) {
		t.Error("evictExpired removed a freshly created, still-valid token")
	}
}
