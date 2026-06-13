package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

// authConfig returns a config with auth enabled so requireAuth is active.
// Auth is triggered by a non-empty Password (no Enabled flag exists).
func authConfig() config.Config {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}
	return cfg
}

func TestRestartReturns501WhenNoRestartFuncConfigured(t *testing.T) {
	server := NewServer(testConfig(), "v-test", events.NewBus(), nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/restart", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

func TestRestartReturns200AndInvokesCallback(t *testing.T) {
	var called atomic.Bool
	restartFn := func() { called.Store(true) }

	server := NewServer(testConfig(), "v-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithRestartFunc(restartFn),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/restart", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "restarting" {
		t.Errorf("body status = %q, want %q", body["status"], "restarting")
	}

	// The callback is invoked after a 500 ms goroutine delay; wait up to 2 s.
	deadline := time.Now().Add(2 * time.Second)
	for !called.Load() {
		if time.Now().After(deadline) {
			t.Fatal("restart callback was not invoked within 2 s")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRestartRequiresAuthWhenEnabled(t *testing.T) {
	var called atomic.Bool
	restartFn := func() { called.Store(true) }

	server := NewServer(authConfig(), "v-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithRestartFunc(restartFn),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/restart", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", rec.Code)
	}
	if called.Load() {
		t.Error("restart callback must not be invoked for unauthenticated request")
	}
}
