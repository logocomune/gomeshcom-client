package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionPersistencePath(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
		want    string
	}{
		{name: "empty disables persistence", dataDir: "", want: ""},
		{name: "data directory", dataDir: "data", want: filepath.Join("data", "http-sessions.json")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sessionPersistencePath(test.dataDir); got != test.want {
				t.Errorf("sessionPersistencePath(%q) = %q, want %q", test.dataDir, got, test.want)
			}
		})
	}
}

func TestSessionStoreLoadsValidSessionAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	firstStore := newSessionStore(path)

	token, _, err := firstStore.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	restartedStore := newSessionStore(path)
	if !restartedStore.valid(token) {
		t.Error("session not valid after store restart")
	}
}

func TestSessionStorePersistsOnlyTokenHashWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	store := newSessionStore(path)

	token, _, err := store.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sessions: %v", err)
	}
	if strings.Contains(string(data), token) {
		t.Error("persisted sessions contain raw token")
	}
	if !strings.Contains(string(data), hashSessionToken(token)) {
		t.Error("persisted sessions do not contain token hash")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat sessions: %v", err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Errorf("session file permissions = %o, want 600", permissions)
	}
}

func TestSessionStoreDeletePersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	store := newSessionStore(path)
	token, _, err := store.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.delete(token); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if newSessionStore(path).valid(token) {
		t.Error("deleted session valid after restart")
	}
}

func TestSessionStoreDoesNotLoadExpiredSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	store := newSessionStore(path)
	token, _, err := store.create(time.Millisecond)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	restartedStore := newSessionStore(path)
	if restartedStore.valid(token) {
		t.Error("expired session valid after restart")
	}
	if len(restartedStore.sessions) != 0 {
		t.Errorf("loaded sessions = %d, want 0", len(restartedStore.sessions))
	}
}

func TestSessionStoreCreateRollsBackOnPersistenceError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("create parent file: %v", err)
	}
	store := newSessionStore(filepath.Join(parentFile, "http-sessions.json"))

	if _, _, err := store.create(time.Hour); err == nil {
		t.Fatal("create error = nil, want persistence error")
	}
	if len(store.sessions) != 0 {
		t.Errorf("sessions after failed create = %d, want 0", len(store.sessions))
	}
}

func TestSessionStoreIgnoresMalformedPersistenceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write malformed sessions: %v", err)
	}

	store := newSessionStore(path)
	if len(store.sessions) != 0 {
		t.Errorf("loaded malformed sessions = %d, want 0", len(store.sessions))
	}
}

func TestSessionStoreEvictExpiredRemovesOnlyStaleTokens(t *testing.T) {
	s := newSessionStore("")

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
	s := newSessionStore("")

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
	s := newSessionStore("")
	ctx, cancel := context.WithCancel(context.Background())
	s.start(ctx)
	cancel()
	// Goroutine must drain and exit; no deadlock or panic.
	time.Sleep(20 * time.Millisecond)
}

func TestSessionStoreEvictExpiredDoesNotRemoveJustCreated(t *testing.T) {
	s := newSessionStore("")

	token, _, err := s.create(time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	s.evictExpired()

	if !s.valid(token) {
		t.Error("evictExpired removed a freshly created, still-valid token")
	}
}
