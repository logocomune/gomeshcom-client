package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/channelshow"
	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

func TestGetChannelShowDefault(t *testing.T) {
	store := newTestChannelShow(t)
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil, WithChannelShow(store))

	request := httptest.NewRequest(http.MethodGet, "/api/channel-show", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var got channelshow.Config
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Mode != channelshow.ModeAll || len(got.Channels) != 0 {
		t.Fatalf("config = %+v, want all", got)
	}
}

func TestUpdateChannelShow(t *testing.T) {
	tests := map[string]struct {
		body         string
		wantStatus   int
		wantMode     string
		wantChannels []string
	}{
		"object allowlist": {
			body:         `{"mode":"allowlist","channels":["*","222","222"]}`,
			wantStatus:   http.StatusOK,
			wantMode:     channelshow.ModeAllowlist,
			wantChannels: []string{"*", "222"},
		},
		"object all": {
			body:         `{"mode":"all","channels":["222"]}`,
			wantStatus:   http.StatusOK,
			wantMode:     channelshow.ModeAll,
			wantChannels: []string{},
		},
		"array shorthand": {
			body:         `["222","22201"]`,
			wantStatus:   http.StatusOK,
			wantMode:     channelshow.ModeAllowlist,
			wantChannels: []string{"222", "22201"},
		},
		"invalid channel": {
			body:       `{"mode":"allowlist","channels":["abc"]}`,
			wantStatus: http.StatusBadRequest,
		},
		"invalid mode": {
			body:       `{"mode":"hidden","channels":["222"]}`,
			wantStatus: http.StatusBadRequest,
		},
		"unknown field": {
			body:       `{"mode":"all","hidden":true}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			store := newTestChannelShow(t)
			server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil, WithChannelShow(store))
			request := httptest.NewRequest(http.MethodPut, "/api/channel-show", bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus != http.StatusOK {
				return
			}
			var got channelshow.Config
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.Mode != test.wantMode {
				t.Fatalf("mode = %q, want %q", got.Mode, test.wantMode)
			}
			if strings.Join(got.Channels, ",") != strings.Join(test.wantChannels, ",") {
				t.Fatalf("channels = %+v, want %+v", got.Channels, test.wantChannels)
			}
		})
	}
}

func TestUpdateChannelShowRequiresAuth(t *testing.T) {
	cfg := testConfig()
	cfg.Auth = config.Auth{
		Username:   "admin",
		Password:   "secret",
		SessionTTL: time.Hour,
		CookieName: "meshcom_session",
	}

	store := newTestChannelShow(t)
	server := NewServer(cfg, "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil, WithChannelShow(store))
	request := httptest.NewRequest(http.MethodPut, "/api/channel-show", bytes.NewBufferString(`{"mode":"all"}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestStreamEventsChannelShowSnapshot(t *testing.T) {
	store := newTestChannelShow(t)
	if _, err := store.Update(channelshow.Config{Mode: channelshow.ModeAllowlist, Channels: []string{"*", "222"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil, WithChannelShow(store))

	body := streamBodyUntil(t, server, "event: channelshow.snapshot")

	if !strings.Contains(body, "event: channelshow.snapshot") {
		t.Fatalf("channelshow.snapshot missing from SSE stream: %q", body)
	}
	if !strings.Contains(body, `"channels":["*","222"]`) {
		t.Fatalf("channelshow snapshot missing channels: %q", body)
	}
}

func TestUpdateChannelShowRejectsTooLargeBody(t *testing.T) {
	store := newTestChannelShow(t)
	server := NewServer(testConfig(), "v0.0.0-test", events.NewBus(), nil, nil, nil, nil, nil, nil, WithChannelShow(store))

	body := bytes.Repeat([]byte("x"), 1<<17) // 128 KB > 64 KB limit
	request := httptest.NewRequest(http.MethodPut, "/api/channel-show", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for oversized body", response.Code)
	}
}

func TestUpdateChannelShowPublishesSSEEvent(t *testing.T) {
	store := newTestChannelShow(t)
	bus := events.NewBus()
	server := NewServer(testConfig(), "v0.0.0-test", bus, nil, nil, nil, nil, nil, nil, WithChannelShow(store))

	// Subscribe to the bus before sending the PUT so we catch the event.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := bus.Subscribe(ctx)

	body := bytes.NewBufferString(`{"mode":"allowlist","channels":["222"]}`)
	request := httptest.NewRequest(http.MethodPut, "/api/channel-show", body)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", response.Code)
	}

	select {
	case ev := <-sub:
		if ev.Type != "channelshow.snapshot" {
			t.Fatalf("event type = %q, want channelshow.snapshot", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("no channelshow.snapshot event received after PUT /api/channel-show")
	}
}

func newTestChannelShow(t *testing.T) *channelshow.Store {
	t.Helper()
	store, err := channelshow.New(filepath.Join(t.TempDir(), "channel_show.json"))
	if err != nil {
		t.Fatalf("channelshow.New: %v", err)
	}
	return store
}
