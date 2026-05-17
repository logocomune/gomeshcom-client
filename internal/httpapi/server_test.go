package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/chatlog"
	"github.com/logocomune/gomeshcom-udp/internal/config"
	"github.com/logocomune/gomeshcom-udp/internal/events"
	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
	"github.com/logocomune/gomeshcom-udp/internal/positions"
	"github.com/logocomune/gomeshcom-udp/internal/receivelog"
	"github.com/logocomune/gomeshcom-udp/internal/sendcache"
	"github.com/logocomune/gomeshcom-udp/internal/udpbridge"
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

func TestHealth(t *testing.T) {
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
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

func TestListPositions(t *testing.T) {
	positionStore := positions.New(t.TempDir() + "/positions.json")
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	positionStore.Update(meshcom.Position{
		Source:    "QQ1ABC-1",
		Latitude:  48.1,
		Longitude: 16.3,
		Altitude:  123,
		RSSI:      -90,
		SNR:       8,
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
		RSSI:      -90,
		SNR:       8,
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
	if err := logger.Append(receivelog.Record{
		ReceivedAt: time.Now().UTC().Add(-time.Minute),
		RemoteAddr: "127.0.0.1:1799",
		Raw:        `{"type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append recent record: %v", err)
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
	body := streamBodyUntil(t, server, "event: packet.received")

	if !strings.Contains(body, "event: packet.received") {
		t.Fatalf("replay event missing from stream body: %q", body)
	}
	if !strings.Contains(body, "QQ1ABC-1") {
		t.Fatalf("recent packet missing from stream body: %q", body)
	}
	if strings.Contains(body, "OLD") {
		t.Fatalf("old packet replayed: %q", body)
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

func streamBodyUntil(t *testing.T, server *Server, marker string) string {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(response, request)
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		body := response.Body.String()
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
