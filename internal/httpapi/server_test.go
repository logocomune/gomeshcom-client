package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/logfmt"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/outbox"
	"github.com/logocomune/gomeshcom-client/internal/positions"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
	"github.com/logocomune/gomeshcom-client/internal/sendcache"
	"github.com/logocomune/gomeshcom-client/internal/udpbridge"
)

// stubBridge is a fake messageSender for tests.
type stubBridge struct {
	calls atomic.Int32
	err   error
}

func (b *stubBridge) SendText(_ context.Context, _, _ string, _ int) error {
	b.calls.Add(1)
	return b.err
}

func TestAPIResponsesDisableCaching(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	tests := []struct {
		name string
		path string
	}{
		{name: "success", path: "/api/health"},
		{name: "error", path: "/api/events?from=bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			assertNoCacheHeaders(t, response.Header())
		})
	}
}

func TestEventStreamDisablesCaching(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	assertNoCacheHeaders(t, response.Header())
}

func TestIndexHTMLDisablesCaching(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	assertIndexNoCacheHeaders(t, response.Header())
}

func TestImmutableStaticResponsesUseLongCache(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/_app/immutable/entry/app.js", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q, want immutable cache", got)
	}
}

func assertIndexNoCacheHeaders(t *testing.T, header http.Header) {
	t.Helper()
	expected := map[string]string{
		"Cache-Control": "no-cache, must-revalidate",
		"Pragma":        "no-cache",
		"Expires":       "0",
	}
	for name, want := range expected {
		if got := header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func assertNoCacheHeaders(t *testing.T, header http.Header) {
	t.Helper()
	expected := map[string]string{
		"Cache-Control": "no-store, no-cache, must-revalidate, max-age=0",
		"Pragma":        "no-cache",
		"Expires":       "0",
	}
	for name, want := range expected {
		if got := header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestHealth(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestRequestLogEnabledLogsStructuredRequest(t *testing.T) {
	var logBuffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(logfmt.New(&logBuffer, slog.LevelDebug)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	cfg := testConfig()
	cfg.RequestLog.Enabled = true
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.RemoteAddr = "198.51.100.12:4321"
	request.Header.Set("CF-Connecting-IP", "203.0.113.10")
	request.Header.Set("X-Real-IP", "198.51.100.99")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	logLine := logBuffer.String()
	for _, want := range []string{
		"http request",
		"method=GET",
		"endpoint=/api/health",
		"status=200",
		"caller_ip=203.0.113.10",
		"started_at=",
		"duration=",
		"duration_ms=",
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("request log = %q, want %q", logLine, want)
		}
	}
}

func TestRequestLogDisabledDoesNotLogRequest(t *testing.T) {
	var logBuffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(logfmt.New(&logBuffer, slog.LevelDebug)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if logBuffer.Len() != 0 {
		t.Fatalf("request log = %q, want empty", logBuffer.String())
	}
}

func TestRequestLogUsesRealIPWhenCloudflareHeaderMissing(t *testing.T) {
	var logBuffer bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(logfmt.New(&logBuffer, slog.LevelDebug)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	cfg := testConfig()
	cfg.RequestLog.Enabled = true
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("X-Real-IP", "198.51.100.99")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if !strings.Contains(logBuffer.String(), "caller_ip=198.51.100.99") {
		t.Fatalf("request log = %q, want X-Real-IP", logBuffer.String())
	}
}

func TestCreateMessage(t *testing.T) {
	tests := map[string]struct {
		body       map[string]string
		bridgeErr  error
		wantStatus int
		wantCalls  int
	}{
		"valid": {
			body:       map[string]string{"dst": "*", "msg": "hello"},
			wantStatus: http.StatusAccepted,
			wantCalls:  1,
		},
		"invalid": {
			body:       map[string]string{"dst": "*", "msg": ""},
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		"bridge error": {
			body:       map[string]string{"dst": "QQ1ABC-1", "msg": "hi"},
			bridgeErr:  fmt.Errorf("udp timeout"),
			wantStatus: http.StatusBadGateway,
			wantCalls:  1,
		},
		"node not yet detected": {
			body:       map[string]string{"dst": "QQ1ABC-1", "msg": "hi"},
			bridgeErr:  udpbridge.ErrNodeNotDetected,
			wantStatus: http.StatusServiceUnavailable,
			wantCalls:  1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(test.body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}

			bridge := &stubBridge{err: test.bridgeErr}
			server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, bridge, nil)
			request := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", response.Code, test.wantStatus, response.Body.String())
			}
			if int(bridge.calls.Load()) != test.wantCalls {
				t.Fatalf("bridge calls = %d, want %d", bridge.calls.Load(), test.wantCalls)
			}
		})
	}
}

func TestCreateMessagePersistsFailedWhenEchoMissing(t *testing.T) {
	cfg := testConfig()
	dir := t.TempDir()
	cfg.ChatLog.Path = dir
	bus := events.NewBus()
	log := chatlog.New(dir, cfg.MyCall)
	server := NewServer(cfg, "v0.0.0-test", bus, nil, nil, log, &stubBridge{}, nil)
	server.outbox = outbox.New(10*time.Millisecond, server.handleOutgoingTimeout)

	body := []byte(`{"dst":"QQ1ABC-1","msg":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusAccepted, response.Body.String())
	}

	var records []chatlog.Record
	deadline := time.After(time.Second)
	for len(records) == 0 {
		var err error
		records, err = log.ReadSince("DM_QQ1ABC-1", time.Time{})
		if err != nil {
			t.Fatalf("ReadSince: %v", err)
		}
		select {
		case <-deadline:
			t.Fatal("failed record not persisted")
		case <-time.After(10 * time.Millisecond):
		}
	}

	if records[0].DeliveryStatus != "failed" {
		t.Fatalf("DeliveryStatus = %q, want failed", records[0].DeliveryStatus)
	}
	if records[0].Direction != "outbound" {
		t.Fatalf("Direction = %q, want outbound", records[0].Direction)
	}
}

func TestCreateMessageDoesNotFailWhenEchoArrives(t *testing.T) {
	cfg := testConfig()
	dir := t.TempDir()
	cfg.ChatLog.Path = dir
	bus := events.NewBus()
	log := chatlog.New(dir, cfg.MyCall)
	server := NewServer(cfg, "v0.0.0-test", bus, nil, nil, log, &stubBridge{}, nil)
	server.outbox = outbox.New(30*time.Millisecond, server.handleOutgoingTimeout)

	body := []byte(`{"dst":"QQ1ABC-1","msg":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusAccepted, response.Body.String())
	}

	bus.Publish(events.Event{
		Type: "packet.received",
		Data: map[string]any{
			"packet": meshcom.TextMessage{
				Source:      cfg.MyCall,
				Destination: "QQ1ABC-1",
				Message:     "hello{5712",
			},
		},
	})

	time.Sleep(60 * time.Millisecond)
	records, err := log.ReadSince("DM_QQ1ABC-1", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %+v, want none", records)
	}
}

func TestAuthProtectedRouteRequiresSession(t *testing.T) {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/chat/list", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(response.Body.String(), "unauthorized") {
		t.Fatalf("body = %q, want unauthorized error", response.Body.String())
	}
}

func TestAuthSessionLifecycle(t *testing.T) {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	loginBody, err := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusNoContent)
	}

	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/api/chat/list", nil)
	protectedReq.AddCookie(cookies[0])
	protectedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(protectedRec, protectedReq)

	if protectedRec.Code != http.StatusOK {
		t.Fatalf("protected status = %d, want %d", protectedRec.Code, http.StatusOK)
	}

	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	logoutReq.AddCookie(cookies[0])
	logoutRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logoutRec.Code, http.StatusNoContent)
	}

	reuseReq := httptest.NewRequest(http.MethodGet, "/api/chat/list", nil)
	reuseReq.AddCookie(cookies[0])
	reuseRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(reuseRec, reuseReq)

	if reuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("reused session status = %d, want %d", reuseRec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRejectsInvalidCredentials(t *testing.T) {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	loginBody, err := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewReader(loginBody))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestGetSessionStatus(t *testing.T) {
	t.Run("auth disabled", func(t *testing.T) {
		server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
		request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		response := httptest.NewRecorder()

		server.Handler().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if !strings.Contains(response.Body.String(), `"authenticated":true`) {
			t.Fatalf("body = %q", response.Body.String())
		}
	})

	t.Run("auth enabled without cookie", func(t *testing.T) {
		cfg := testConfig()
		cfg.Auth = config.Auth{
			Username:   "admin",
			Password:   "secret",
			SessionTTL: time.Hour,
			CookieName: "meshcom_session",
		}

		server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
		request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		response := httptest.NewRecorder()

		server.Handler().ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
		}
		if !strings.Contains(response.Body.String(), `"required":true`) {
			t.Fatalf("body = %q", response.Body.String())
		}
	})
}

func TestCreateMessageDedup(t *testing.T) {
	bridge := &stubBridge{}
	sc := sendcache.New(100 * time.Millisecond)
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, bridge, sc)

	post := func(dst, msg string) int {
		body, _ := json.Marshal(map[string]string{"dst": dst, "msg": msg})
		req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	// First send: accepted.
	if got := post("*", "hello"); got != http.StatusAccepted {
		t.Fatalf("first send status = %d, want 202", got)
	}
	if bridge.calls.Load() != 1 {
		t.Fatalf("bridge calls = %d, want 1", bridge.calls.Load())
	}

	// Duplicate within TTL: 429.
	if got := post("*", "hello"); got != http.StatusTooManyRequests {
		t.Fatalf("duplicate status = %d, want 429", got)
	}
	if bridge.calls.Load() != 1 {
		t.Fatalf("bridge calls after duplicate = %d, still want 1", bridge.calls.Load())
	}

	// Different dst: accepted.
	if got := post("QQ1ABC-1", "hello"); got != http.StatusAccepted {
		t.Fatalf("different dst status = %d, want 202", got)
	}

	// Different msg: accepted.
	if got := post("*", "world"); got != http.StatusAccepted {
		t.Fatalf("different msg status = %d, want 202", got)
	}

	// After TTL expiry: accepted again.
	time.Sleep(120 * time.Millisecond)
	if got := post("*", "hello"); got != http.StatusAccepted {
		t.Fatalf("post-TTL status = %d, want 202", got)
	}
}

func intPtr(value int) *int { return &value }

func TestListPositions(t *testing.T) {
	positionStore := positions.New(t.TempDir() + "/positions.json")
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	positionStore.Update(meshcom.Position{
		Source:    "QQ1ABC-1",
		Latitude:  48.1,
		Longitude: 16.3,
		Altitude:  123,
		RSSI:      intPtr(-90),
		SNR:       intPtr(8),
	}, seenAt)

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), positionStore, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/positions", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body map[string]positions.Record
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode positions: %v", err)
	}
	if body["QQ1ABC-1"].Longitude != 16.3 {
		t.Fatalf("positions = %+v", body)
	}
	if body["QQ1ABC-1"].LastDirectSeen == nil || !body["QQ1ABC-1"].LastDirectSeen.Equal(seenAt) {
		t.Fatalf("positions lastdirectseen = %+v", body["QQ1ABC-1"].LastDirectSeen)
	}
}

func TestListConversations(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")
	log.Append(meshcom.TextMessage{
		Destination: "*",
		Message:     "hello",
	}, testTime())

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/chat/list", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body []chatlog.Conversation
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode conversations: %v", err)
	}
	if len(body) != 1 || body[0].ID != "P_broadcast" {
		t.Fatalf("conversations = %+v", body)
	}
}

func TestGetConversation(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")
	log.Append(meshcom.TextMessage{
		Destination: "*",
		Message:     "hello",
	}, testTime())

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)

	// Use the correct path format for Mux path values in tests
	// Actually ServeMux in Go 1.22+ needs the path to match the pattern
	request := httptest.NewRequest(http.MethodGet, "/api/chat/P_broadcast", nil)
	request.SetPathValue("conversation", "P_broadcast")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", response.Code, http.StatusOK, response.Body.String())
	}

	var body []chatlog.Record
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	if len(body) != 1 || body[0].Msg != "hello" {
		t.Fatalf("records = %+v", body)
	}
}

func TestGetConversationInvalid(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/chat/invalid!", nil)
	request.SetPathValue("conversation", "invalid!")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestGetConversationWithHours(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")

	now := time.Now().UTC()
	log.Append(meshcom.TextMessage{Destination: "*", Message: "recent"}, now.Add(-10*time.Minute))
	log.Append(meshcom.TextMessage{Destination: "*", Message: "old"}, now.Add(-2*time.Hour))

	cfg := testConfig()
	cfg.ChatLog.HistoryWindow = time.Hour
	cfg.ChatLog.MaxHistoryWindow = 24 * time.Hour

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)

	// Default window (1h) -> only "recent"
	req1 := httptest.NewRequest(http.MethodGet, "/api/chat/P_broadcast", nil)
	req1.SetPathValue("conversation", "P_broadcast")
	rec1 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec1, req1)

	var body1 []chatlog.Record
	json.Unmarshal(rec1.Body.Bytes(), &body1)
	if len(body1) != 1 {
		t.Errorf("default window count = %d, want 1", len(body1))
	}

	// Custom window (3h) -> both
	req2 := httptest.NewRequest(http.MethodGet, "/api/chat/P_broadcast?hours=3", nil)
	req2.SetPathValue("conversation", "P_broadcast")
	rec2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec2, req2)

	var body2 []chatlog.Record
	json.Unmarshal(rec2.Body.Bytes(), &body2)
	if len(body2) != 2 {
		t.Errorf("custom window count = %d, want 2", len(body2))
	}
}

func TestGetConversationUsesThirtyDaysForDMDefault(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")

	now := time.Now().UTC()
	log.Append(meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "QQ0QQ-1", Message: "dm recent"}, now.Add(-29*24*time.Hour))
	log.Append(meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "QQ0QQ-1", Message: "dm old"}, now.Add(-31*24*time.Hour))
	log.Append(meshcom.TextMessage{Destination: "*", Message: "broadcast old"}, now.Add(-2*time.Hour))

	cfg := testConfig()
	cfg.ChatLog.HistoryWindow = time.Hour
	cfg.ChatLog.MaxHistoryWindow = 30 * 24 * time.Hour

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)

	dmRequest := httptest.NewRequest(http.MethodGet, "/api/chat/DM_QQ1ABC-1", nil)
	dmRequest.SetPathValue("conversation", "DM_QQ1ABC-1")
	dmResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(dmResponse, dmRequest)

	var dmBody []chatlog.Record
	if err := json.Unmarshal(dmResponse.Body.Bytes(), &dmBody); err != nil {
		t.Fatalf("decode dm records: %v", err)
	}
	if len(dmBody) != 1 || dmBody[0].Msg != "dm recent" {
		t.Fatalf("dm records = %+v", dmBody)
	}

	broadcastRequest := httptest.NewRequest(http.MethodGet, "/api/chat/P_broadcast", nil)
	broadcastRequest.SetPathValue("conversation", "P_broadcast")
	broadcastResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(broadcastResponse, broadcastRequest)

	var broadcastBody []chatlog.Record
	if err := json.Unmarshal(broadcastResponse.Body.Bytes(), &broadcastBody); err != nil {
		t.Fatalf("decode broadcast records: %v", err)
	}
	if len(broadcastBody) != 0 {
		t.Fatalf("broadcast records = %+v, want empty", broadcastBody)
	}
}

func TestSPARouting(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	// Requesting root should serve SPA
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec2, req2)
	// Status should be 200 or 404 depending on if FS is empty, but not 500
	if rec2.Code == http.StatusInternalServerError {
		t.Errorf("root status = %d", rec2.Code)
	}

	// Unknown non-API path should also return 200 (fallback to index.html)
	req3 := httptest.NewRequest(http.MethodGet, "/some/ui/route", nil)
	rec3 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec3, req3)
	if rec3.Code == http.StatusInternalServerError {
		t.Errorf("spa fallback status = %d", rec3.Code)
	}
}

func TestStreamEventsHeartbeatAndIdentity(t *testing.T) {
	positionStore := positions.New(t.TempDir() + "/positions.json")
	positionStore.Update(meshcom.Position{
		Source:    "QQ1ABC-1",
		Latitude:  48.1,
		Longitude: 16.3,
		RSSI:      intPtr(-90),
		SNR:       intPtr(8),
	}, testTime())

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), positionStore, nil, nil, nil, nil)
	body := streamBodyUntil(t, server, "event: positions.snapshot")
	if !strings.Contains(body, "event: positions.snapshot") {
		t.Fatalf("snapshot event missing from stream body: %q", body)
	}
}

func TestStreamEventsStartsWithHeartbeat(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	body := streamBodyUntil(t, server, "event: heartbeat")
	if !strings.HasPrefix(body, "event: heartbeat\ndata: {}\n\n") {
		t.Fatalf("stream body prefix = %q, want heartbeat event", body)
	}
}

func TestStreamEventsRequiresSessionWhenAuthEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}

	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestStreamEventsSendsStationIdentity(t *testing.T) {
	cfg := testConfig()
	cfg.MyCall = "QQ1ABC-7"
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	body := streamBodyUntil(t, server, "event: station.identity")
	if !strings.Contains(body, `"callsign":"QQ1ABC-7"`) {
		t.Fatalf("station identity missing callsign: %q", body)
	}
}

func TestStreamEventsSendsStationIdentityTxDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.MyCall = "QQ1ABC-7"
	cfg.Send.DisableTx = true
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	body := streamBodyUntil(t, server, "event: station.identity")
	if !strings.Contains(body, `"txDisabled":true`) {
		t.Fatalf("station identity missing txDisabled:true: %q", body)
	}
}

func TestStreamEventsStationIdentityTxEnabledByDefault(t *testing.T) {
	cfg := testConfig()
	cfg.MyCall = "QQ1ABC-7"
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)

	body := streamBodyUntil(t, server, "event: station.identity")
	if !strings.Contains(body, `"txDisabled":false`) {
		t.Fatalf("station identity missing txDisabled:false: %q", body)
	}
	if !strings.Contains(body, `"forwardTargetCount":0`) {
		t.Fatalf("station identity missing forwardTargetCount:0: %q", body)
	}
}

func TestStreamEventsReplaysRecentReceiveLogPackets(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "raw")
	logger := receivelog.New(receivelog.Config{Enabled: true, Path: dir})
	recentTime := time.Now().UTC().Add(-2 * time.Minute)
	if err := logger.Append(receivelog.Record{
		ReceivedAt: recentTime,
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append recent record: %v", err)
	}
	if err := logger.Append(receivelog.Record{
		ReceivedAt: time.Now().UTC().Add(-time.Minute),
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"src_type":"node","type":"pos","src":"POS1","msg":"","lat":48,"lat_dir":"N","long":16,"long_dir":"E","aprs_symbol":"#","aprs_symbol_group":"/","hw_id":"MAC","msg_id":"ABC","alt":123,"batt":85,"firmware":"4.35","fw_sub":"p","rssi":-90,"snr":8}`,
		PacketType: "pos",
	}); err != nil {
		t.Fatalf("append recent position record: %v", err)
	}
	if err := logger.Append(receivelog.Record{
		ReceivedAt: time.Now().UTC().Add(-7 * time.Hour),
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"OLD","dst":"*","msg":"old"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append old record: %v", err)
	}

	cfg := testConfig()
	cfg.ReceiveLog = config.ReceiveLog{
		Enabled:      true,
		Path:         dir,
		ReplayWindow: 6 * time.Hour,
	}
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, logger, nil, nil, nil)
	body := streamBodyUntil(t, server, "QQ1ABC-1")

	if !strings.Contains(body, "event: packet.received") {
		t.Fatalf("replay event missing from stream body: %q", body)
	}
	if !strings.Contains(body, "QQ1ABC-1") {
		t.Fatalf("recent packet missing from stream body: %q", body)
	}
	if !strings.Contains(body, `"replay":true`) {
		t.Fatalf("replay marker missing from stream body: %q", body)
	}
	if !strings.Contains(body, `"received_at":"`+recentTime.Format(time.RFC3339Nano)+`"`) {
		t.Fatalf("replay received_at missing from stream body: %q", body)
	}
	if strings.Contains(body, "OLD") {
		t.Fatalf("old packet replayed: %q", body)
	}
}

func TestStreamEventsReplayFromQueryCappedByReplayWindow(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "raw")
	logger := receivelog.New(receivelog.Config{Enabled: true, Path: dir})
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	if err := logger.Append(receivelog.Record{
		ReceivedAt: oldTime,
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"OLDIN","dst":"*","msg":"old"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append old record: %v", err)
	}

	cfg := testConfig()
	cfg.ReceiveLog = config.ReceiveLog{
		Enabled:      true,
		Path:         dir,
		ReplayWindow: time.Hour,
	}
	bus := events.NewBus()
	server := NewServer(cfg, "v0.0.0-test", bus, nil, logger, nil, nil, nil)
	from := oldTime.Add(-time.Minute).Format(time.RFC3339Nano)

	go func() {
		time.Sleep(50 * time.Millisecond)
		bus.Publish(events.Event{Type: "packet.received", Data: "LIVEMARKER"})
	}()

	// Requesting 'from' 2h1m ago, but ReplayWindow is 1h. It should cap to 1h, so OLDIN (2h ago) is not replayed.
	body := streamBodyUntilPath(t, server, "/api/events?from="+from, "LIVEMARKER")

	if strings.Contains(body, "OLDIN") {
		t.Fatalf("packet older than ReplayWindow was replayed: %q", body)
	}
}

func TestStreamEventsReplayFromQueryWithinReplayWindow(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "raw")
	logger := receivelog.New(receivelog.Config{Enabled: true, Path: dir})

	// Packet 1: 90 minutes ago (within 2h ReplayWindow)
	packet1Time := time.Now().UTC().Add(-90 * time.Minute)
	if err := logger.Append(receivelog.Record{
		ReceivedAt: packet1Time,
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"PACKET1","dst":"*","msg":"p1"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append record 1: %v", err)
	}

	// Packet 2: 30 minutes ago (within 2h ReplayWindow)
	packet2Time := time.Now().UTC().Add(-30 * time.Minute)
	if err := logger.Append(receivelog.Record{
		ReceivedAt: packet2Time,
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"PACKET2","dst":"*","msg":"p2"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append record 2: %v", err)
	}

	cfg := testConfig()
	cfg.ReceiveLog = config.ReceiveLog{
		Enabled:      true,
		Path:         dir,
		ReplayWindow: 2 * time.Hour,
	}
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, logger, nil, nil, nil)

	// Query 'from' 1 hour ago. Only Packet 2 (30m ago) should be replayed. Packet 1 (90m ago) should be filtered out.
	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	body := streamBodyUntilPath(t, server, "/api/events?from="+from, "PACKET2")

	if strings.Contains(body, "PACKET1") {
		t.Fatalf("packet older than 'from' query parameter was replayed: %q", body)
	}
	if !strings.Contains(body, "PACKET2") {
		t.Fatalf("packet newer than 'from' query parameter was not replayed: %q", body)
	}
}

func TestStreamEventsRejectsInvalidReplayFromQuery(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/events?from=bad", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "invalid from timestamp") {
		t.Fatalf("body = %q, want invalid from error", response.Body.String())
	}
}

func TestDeleteConversation(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")
	log.Append(meshcom.TextMessage{Destination: "*", Message: "hello"}, testTime())

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/chat/P_broadcast", nil)
	request.SetPathValue("conversation", "P_broadcast")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", response.Code, response.Body.String())
	}

	convs, _ := log.List()
	for _, c := range convs {
		if c.ID == "P_broadcast" {
			t.Fatal("P_broadcast still in List after delete")
		}
	}
}

func TestDeleteConversationInvalidID(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/chat/invalid!", nil)
	request.SetPathValue("conversation", "invalid!")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestDeleteConversationMissingFile(t *testing.T) {
	log := chatlog.New(t.TempDir(), "QQ0QQ-1")
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/chat/P_999", nil)
	request.SetPathValue("conversation", "P_999")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (idempotent)", response.Code)
	}
}

func TestDeleteBroadcast(t *testing.T) {
	dir := t.TempDir()
	log := chatlog.New(dir, "QQ0QQ-1")
	log.Append(meshcom.TextMessage{Destination: "*", Message: "test"}, testTime())

	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, log, nil, nil)
	request := httptest.NewRequest(http.MethodDelete, "/api/chat/P_broadcast", nil)
	request.SetPathValue("conversation", "P_broadcast")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}

	records, _ := log.ReadSince("P_broadcast", time.Time{})
	if len(records) != 0 {
		t.Fatalf("records after broadcast delete = %d, want 0", len(records))
	}
}

func TestServerClose(t *testing.T) {
	bus := events.NewBus()
	server := NewServer(testConfig(), "v0.0.0-test", bus, nil, nil, nil, nil, nil)

	// Register a message in outbox
	server.outbox.Register("SRC", "DST", "HELLO", time.Now())

	// Close the server to cancel the background watch goroutine
	server.Close()
	time.Sleep(50 * time.Millisecond) // Let the cancellation goroutine run

	// Publish the packet.received event which would normally confirm and remove the message from outbox
	bus.Publish(events.Event{
		Type: "packet.received",
		Data: map[string]any{
			"packet": meshcom.TextMessage{
				Source:      "SRC",
				Destination: "DST",
				Message:     "HELLO",
			},
		},
	})
	time.Sleep(50 * time.Millisecond)

	// Confirm that the message in outbox was NOT confirmed (still exists), because the watch goroutine was stopped.
	if !server.outbox.Confirm("SRC", "DST", "HELLO") {
		t.Error("expected message to still be pending in outbox after Close()")
	}
}

type safeResponseWriter struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
}

func (s *safeResponseWriter) Header() http.Header {
	return s.rec.Header()
}

func (s *safeResponseWriter) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Write(b)
}

func (s *safeResponseWriter) WriteHeader(statusCode int) {
	s.rec.WriteHeader(statusCode)
}

func (s *safeResponseWriter) Flush() {
	s.rec.Flush()
}

func (s *safeResponseWriter) BodyString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Body.String()
}

func streamBodyUntil(t *testing.T, server *Server, marker string) string {
	t.Helper()
	return streamBodyUntilPath(t, server, "/api/events", marker)
}

func streamBodyUntilPath(t *testing.T, server *Server, path string, marker string) string {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, path, nil)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	safeWriter := &safeResponseWriter{rec: response}

	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(safeWriter, request)
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		body := safeWriter.BodyString()
		if strings.Contains(body, marker) {
			cancel()
			<-done
			return body
		}

		select {
		case <-deadline:
			t.Fatalf("stream marker %q missing from body: %q", marker, body)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func testConfig() config.Config {
	return config.Config{
		MyCall:           "QQ0QQ-1",
		MaxMessageLength: 149,
		ChatLog: config.ChatLog{
			HistoryWindow:    24 * time.Hour,
			MaxHistoryWindow: 7 * 24 * time.Hour,
		},
	}
}

func testTime() time.Time {
	return time.Now().UTC().Add(-time.Hour)
}
