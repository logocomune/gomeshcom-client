package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/station"
)

func TestGetMyCall(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")
	srv := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	req := httptest.NewRequest(http.MethodGet, "/api/adm/configs/my-call", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body stationIdentityEvent
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Callsign != "IU5PMP-1" {
		t.Errorf("callsign = %q, want IU5PMP-1", body.Callsign)
	}
}

func TestUpdateMyCallAcceptsValid(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")
	bus := events.NewBus()
	srv := NewServer(testConfig(), "v0.0.0-test", bus, nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	// Subscribe to the event bus to verify SSE broadcast.
	sub := bus.Subscribe(t.Context())

	payload, _ := json.Marshal(map[string]string{"callsign": "QQ0QQ-2"})
	req := httptest.NewRequest(http.MethodPut, "/api/adm/configs/my-call", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body stationIdentityEvent
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Callsign != "QQ0QQ-2" {
		t.Errorf("response callsign = %q, want QQ0QQ-2", body.Callsign)
	}
	if id.Current() != "QQ0QQ-2" {
		t.Errorf("identity.Current() = %q, want QQ0QQ-2 after update", id.Current())
	}

	// Verify station.identity event published on the bus.
	select {
	case ev := <-sub:
		if ev.Type != "station.identity" {
			t.Errorf("event type = %q, want station.identity", ev.Type)
		}
		data, ok := ev.Data.(stationIdentityEvent)
		if !ok {
			t.Fatalf("event data type = %T, want stationIdentityEvent", ev.Data)
		}
		if data.Callsign != "QQ0QQ-2" {
			t.Errorf("event callsign = %q, want QQ0QQ-2", data.Callsign)
		}
	default:
		t.Error("no station.identity event published on bus")
	}
}

func TestUpdateMyCallRejectsInvalid(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")
	srv := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	payload, _ := json.Marshal(map[string]string{"callsign": "!!"})
	req := httptest.NewRequest(http.MethodPut, "/api/adm/configs/my-call", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if id.Current() != "IU5PMP-1" {
		t.Errorf("identity unchanged after rejection: got %q", id.Current())
	}
}

func TestUpdateMyCallInvalidJSON(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")
	srv := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	req := httptest.NewRequest(http.MethodPut, "/api/adm/configs/my-call", bytes.NewReader([]byte("notjson")))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetMyCallRequiresAuth(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.Username = "admin"
	cfg.Auth.Password = "secret"
	cfg.Auth.SessionTTL = 24 * 60 * 60 * 1000000000 // 24h in nanoseconds
	cfg.Auth.CookieName = "meshcom_session"

	id := station.NewInMemory("IU5PMP-1")
	srv := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	req := httptest.NewRequest(http.MethodGet, "/api/adm/configs/my-call", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	// Auth enabled → unauthenticated request must be rejected.
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when auth is enabled", rec.Code)
	}
}

func TestUpdateMyCallPersistsBeforeResponding(t *testing.T) {
	dir := t.TempDir()
	id, err := station.New(station.DefaultPath(dir), "IU5PMP-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	payload, _ := json.Marshal(map[string]string{"callsign": "QQ0QQ-2"})
	req := httptest.NewRequest(http.MethodPut, "/api/adm/configs/my-call", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// File must exist after response — no crash window.
	id2, err := station.New(station.DefaultPath(dir), "FALLBACK-1")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if id2.Current() != "QQ0QQ-2" {
		t.Errorf("reloaded = %q, want QQ0QQ-2 (should be persisted before response)", id2.Current())
	}
}

func TestUpdateMyCallNormalizesInput(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")
	srv := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil,
		WithStationIdentity(id))

	payload, _ := json.Marshal(map[string]string{"callsign": " iu5pmp-2 "})
	req := httptest.NewRequest(http.MethodPut, "/api/adm/configs/my-call", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if id.Current() != "IU5PMP-2" {
		t.Errorf("identity.Current() = %q, want IU5PMP-2", id.Current())
	}
}
