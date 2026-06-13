package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

// fullTestConfig returns a valid config suitable for configapi tests.
func fullTestConfig() config.Config {
	return config.Config{
		HTTPAddr:         "127.0.0.1:8080",
		UDPListenAddr:    "0.0.0.0:1799",
		MyCall:           "QQ0QQ-1",
		DataDir:          "./data",
		MaxMessageLength: 149,
		LogLevel:         "info",
		ChatLog: config.ChatLog{
			Path:             "./data/chat",
			HistoryWindow:    24 * time.Hour,
			MaxHistoryWindow: 720 * time.Hour,
		},
		ReceiveLog: config.ReceiveLog{
			Enabled:       true,
			Path:          "./data/raw",
			RetentionDays: 365,
			ReplayWindow:  time.Hour,
		},
		Stats: config.Stats{
			Enabled:       true,
			Path:          "./data/stats/stats.json",
			RetentionDays: 30,
		},
		Send:       config.Send{DedupTTL: 2 * time.Second},
		Auth:       config.Auth{SessionTTL: 24 * time.Hour, CookieName: "meshcom_session"},
		RequestLog: config.RequestLog{Enabled: false},
	}
}

func configTestServer(cfg config.Config, env config.EnvOverrides, tomlDir string) *Server {
	tomlPath := ""
	if tomlDir != "" {
		tomlPath = filepath.Join(tomlDir, "gomeshcomd.toml")
	}
	return NewServer(cfg, "v-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithEnvOverrides(env),
		WithTomlPath(tomlPath),
	)
}

// -------- buildConfigResponse (unit tests, no HTTP) --------

func TestBuildConfigResponsePasswordMasked(t *testing.T) {
	cfg := fullTestConfig()
	cfg.Auth.Password = "supersecret"
	resp := buildConfigResponse(cfg, config.EnvOverrides{}, "", time.Time{})
	if resp.Auth.Password.Value == "supersecret" {
		t.Error("auth.password must not be the real value")
	}
	if resp.Auth.Password.Value != passwordMask {
		t.Errorf("auth.password.value = %v, want %q", resp.Auth.Password.Value, passwordMask)
	}
}

func TestBuildConfigResponseEmptyPasswordNotMasked(t *testing.T) {
	cfg := fullTestConfig()
	resp := buildConfigResponse(cfg, config.EnvOverrides{}, "", time.Time{})
	if resp.Auth.Password.Value != "" {
		t.Errorf("auth.password.value = %v, want empty string", resp.Auth.Password.Value)
	}
}

func TestBuildConfigResponseEnvOverrideMarked(t *testing.T) {
	cfg := fullTestConfig()
	env := config.EnvOverrides{"MY_CALL": true}
	resp := buildConfigResponse(cfg, env, "", time.Time{})
	if !resp.MyCall.EnvOverride {
		t.Error("my_call.env_override should be true when env var is set")
	}
	if resp.LogLevel.EnvOverride {
		t.Error("log_level.env_override should be false when env var not set")
	}
}

func TestBuildConfigResponseRequiresRestartFlag(t *testing.T) {
	cfg := fullTestConfig()
	resp := buildConfigResponse(cfg, config.EnvOverrides{}, "", time.Time{})
	if !resp.HTTPAddr.RequiresRestart {
		t.Error("http_addr.requires_restart should be true")
	}
	if resp.MyCall.RequiresRestart {
		t.Error("my_call.requires_restart should be false (live-apply)")
	}
}

// -------- GET /api/config (HTTP, no auth) --------

func TestGetConfigHTTPReturnsOK(t *testing.T) {
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp configResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// -------- PUT /api/config --------

func TestUpdateConfigPersistsToToml(t *testing.T) {
	dir := t.TempDir()
	cfg := fullTestConfig()
	cfg.MyCall = "QQ0QQ-1"
	cfg.LogLevel = "info"
	server := configTestServer(cfg, config.EnvOverrides{}, dir)

	body, _ := json.Marshal(map[string]any{"log_level": "debug"})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// TOML file should exist and contain the updated log_level.
	data, err := os.ReadFile(filepath.Join(dir, "gomeshcomd.toml"))
	if err != nil {
		t.Fatalf("toml file not created: %v", err)
	}
	content := string(data)
	if !contains(content, `log_level = "debug"`) {
		t.Errorf("expected log_level = \"debug\" in TOML; got:\n%s", content)
	}
}

func TestUpdateConfigInvalidValueRejected(t *testing.T) {
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	body, _ := json.Marshal(map[string]any{"log_level": "trace"}) // invalid
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateConfigEnvLockedFieldRejected(t *testing.T) {
	cfg := fullTestConfig()
	env := config.EnvOverrides{"MY_CALL": true}
	server := configTestServer(cfg, env, "")

	body, _ := json.Marshal(map[string]any{"my_call": "IU5PMP-1"})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestUpdateConfigPasswordMaskIgnored(t *testing.T) {
	// Verify that sending the mask sentinel does not update the password.
	// Auth is disabled (no username/password) so the endpoint is reachable.
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	body, _ := json.Marshal(map[string]any{
		"auth": map[string]any{"password": passwordMask},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Password must remain empty — mask sentinel must not be stored.
	if server.cfg.Auth.Password != "" {
		t.Errorf("password = %q, want empty (mask sentinel must not update)", server.cfg.Auth.Password)
	}
}

func TestUpdateConfigInvalidDurationRejected(t *testing.T) {
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	body, _ := json.Marshal(map[string]any{"receive_log": map[string]any{"replay_window": "notaduration"}})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateConfigInvalidJSONRejected(t *testing.T) {
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpdateConfigRequiresRestartInResponse(t *testing.T) {
	cfg := fullTestConfig()
	server := configTestServer(cfg, config.EnvOverrides{}, "")

	body, _ := json.Marshal(map[string]any{"http_addr": "127.0.0.1:9090"})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		RequiresRestart bool `json:"requires_restart"`
	}
	json.NewDecoder(rec.Body).Decode(&resp) //nolint: errcheck
	if !resp.RequiresRestart {
		t.Error("requires_restart should be true when http_addr changed")
	}
}

func TestUpdateConfigMyCallLiveApply(t *testing.T) {
	cfg := fullTestConfig()
	cfg.MyCall = "QQ0QQ-1"
	bus := events.NewBus()
	ctx := t.Context()
	_ = ctx
	subscriber := bus.Subscribe(ctx)

	server := NewServer(cfg, "v-test", bus, nil, nil, nil, nil, nil, nil,
		WithEnvOverrides(config.EnvOverrides{}),
	)

	body, _ := json.Marshal(map[string]any{"my_call": "IU5PMP-1"})
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Effective callsign must be updated in server.
	if server.cfg.MyCall != "IU5PMP-1" {
		t.Errorf("server.cfg.MyCall = %q, want IU5PMP-1", server.cfg.MyCall)
	}

	// SSE event must have been published.
	select {
	case ev := <-subscriber:
		if ev.Type != "station.identity" {
			t.Errorf("event type = %q, want station.identity", ev.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("no station.identity SSE event published")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
