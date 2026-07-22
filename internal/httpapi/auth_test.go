package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestMemorySessionStoreCreateValidDelete(t *testing.T) {
	store := newMemorySessionStore()
	token, _, err := store.create(time.Hour)
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	if !store.valid(token) {
		t.Fatal("created session is not valid")
	}
	if err := store.delete(token); err != nil {
		t.Fatalf("delete() error = %v", err)
	}
	if store.valid(token) {
		t.Fatal("deleted session is still valid")
	}
}

func TestSessionStoreEvictExpiredRemovesOnlyStaleTokens(t *testing.T) {
	s := newMemorySessionStore()

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
	_, validPresent := s.sessions[hashSessionToken(validToken)]
	s.mu.Unlock()

	if stalePresent {
		t.Error("evictExpired: stale token still present after eviction")
	}
	if !validPresent {
		t.Error("evictExpired: valid token incorrectly removed")
	}
}

func TestSessionStoreEvictExpiredClearsAllExpired(t *testing.T) {
	s := newMemorySessionStore()

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
	s := newMemorySessionStore()
	ctx, cancel := context.WithCancel(context.Background())
	s.start(ctx)
	cancel()
	time.Sleep(20 * time.Millisecond)
}

func TestSessionStoreEvictExpiredDoesNotRemoveJustCreated(t *testing.T) {
	s := newMemorySessionStore()

	token, _, err := s.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	s.evictExpired()

	if !s.valid(token) {
		t.Error("evictExpired removed a freshly created, still-valid token")
	}
}
