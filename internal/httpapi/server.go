package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/chatlog"
	"github.com/logocomune/gomeshcom-udp/internal/config"
	"github.com/logocomune/gomeshcom-udp/internal/events"
	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
	"github.com/logocomune/gomeshcom-udp/internal/positions"
	"github.com/logocomune/gomeshcom-udp/internal/receivelog"
	"github.com/logocomune/gomeshcom-udp/internal/sendcache"
	"github.com/logocomune/gomeshcom-udp/internal/udpbridge"
	"github.com/logocomune/gomeshcom-udp/internal/webui"
)

type stationIdentityEvent struct {
	Callsign           string `json:"callsign"`
	Version            string `json:"version,omitempty"`
	TxDisabled         bool   `json:"txDisabled"`
	ForwardTargetCount int    `json:"forwardTargetCount"`
}

// messageSender abstracts udpbridge.Bridge.SendText for testing.
type messageSender interface {
	SendText(ctx context.Context, destination, message string, maxLength int) error
}

type Server struct {
	cfg        config.Config
	version    string
	bus        *events.Bus
	positions  *positions.Store
	receiveLog *receivelog.Logger
	chatLog    *chatlog.Logger
	bridge     messageSender
	sendCache  *sendcache.Cache
	sessions   *sessionStore
}

func NewServer(cfg config.Config, version string, bus *events.Bus, positionStore *positions.Store, receiveLog *receivelog.Logger, chatLog *chatlog.Logger, bridge messageSender, sc *sendcache.Cache) *Server {
	server := &Server{
		cfg:        cfg,
		version:    version,
		bus:        bus,
		positions:  positionStore,
		receiveLog: receiveLog,
		chatLog:    chatLog,
		bridge:     bridge,
		sendCache:  sc,
	}
	if authEnabled(cfg) {
		server.sessions = newSessionStore()
	}
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/session", s.sessionStatus)
	mux.HandleFunc("POST /api/session", s.createSession)
	mux.HandleFunc("DELETE /api/session", s.deleteSession)
	mux.Handle("GET /api/positions", requireAuth(http.HandlerFunc(s.listPositions), s))
	mux.Handle("POST /api/messages", requireAuth(http.HandlerFunc(s.createMessage), s))
	mux.Handle("GET /api/events", requireAuth(http.HandlerFunc(s.streamEvents), s))
	mux.Handle("GET /api/chat/list", requireAuth(http.HandlerFunc(s.listConversations), s))
	mux.Handle("GET /api/chat/{conversation}", requireAuth(http.HandlerFunc(s.getConversation), s))
	mux.Handle("DELETE /api/chat/{conversation}", requireAuth(http.HandlerFunc(s.deleteConversation), s))
	mux.Handle("/", spaHandler(webui.FS()))
	return mux
}

func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fsys, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path — serve SPA entry point so client-side routing works.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version, "callsign": s.cfg.MyCall})
}

func (s *Server) listPositions(w http.ResponseWriter, _ *http.Request) {
	if s.positions == nil {
		writeJSON(w, http.StatusOK, map[string]positions.Record{})
		return
	}

	writeJSON(w, http.StatusOK, s.positions.Snapshot())
}

func (s *Server) createMessage(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Destination string `json:"dst"`
		Message     string `json:"msg"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	outgoing, err := meshcom.NewOutgoingText(request.Destination, request.Message, s.cfg.MaxMessageLength)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.sendCache != nil && !s.sendCache.Reserve(outgoing.Destination, outgoing.Message) {
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":          "duplicate",
			"retry_after_ms": 2000,
		})
		return
	}

	if s.bridge == nil {
		writeError(w, http.StatusServiceUnavailable, "bridge not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.bridge.SendText(ctx, outgoing.Destination, outgoing.Message, s.cfg.MaxMessageLength); err != nil {
		slog.Error("udp send failed", "error", err)
		if errors.Is(err, udpbridge.ErrNodeNotDetected) {
			writeError(w, http.StatusServiceUnavailable, "node not yet detected")
			return
		}
		writeError(w, http.StatusBadGateway, "send failed")
		return
	}
	slog.Info("message sent", "dst", outgoing.Destination, "msg", outgoing.Message)

	writeJSON(w, http.StatusAccepted, outgoing)
}

func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	subscriber := s.bus.Subscribe(r.Context())
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	if err := writeHeartbeat(w); err != nil {
		return
	}
	flusher.Flush()

	forwardTargets, _ := config.ParseForwardTargets(s.cfg.Forward.Targets)
	if err := writeSSE(w, events.Event{Type: "station.identity", Data: stationIdentityEvent{
		Callsign:           s.cfg.MyCall,
		Version:            s.version,
		TxDisabled:         s.cfg.Send.DisableTx,
		ForwardTargetCount: len(forwardTargets),
	}}); err != nil {
		return
	}
	flusher.Flush()

	if s.positions != nil {
		if err := writeSSE(w, events.Event{Type: "positions.snapshot", Data: s.positions.Snapshot()}); err != nil {
			return
		}
		flusher.Flush()
	}

	if err := s.replayRecentPackets(w, flusher); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-subscriber:
			if !ok {
				return
			}
			if err := writeSSE(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if err := writeHeartbeat(w); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) listConversations(w http.ResponseWriter, _ *http.Request) {
	if s.chatLog == nil {
		writeJSON(w, http.StatusOK, []chatlog.Conversation{})
		return
	}
	convs, err := s.chatLog.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list conversations: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, convs)
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("conversation")
	if !chatlog.ValidConversationID(id) {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}

	defaultHours := s.cfg.ChatLog.HistoryWindow.Hours()
	maxHours := s.cfg.ChatLog.MaxHistoryWindow.Hours()

	hours := defaultHours
	if raw := r.URL.Query().Get("hours"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid hours parameter")
			return
		}
		hours = parsed
	}
	if hours > maxHours {
		hours = maxHours
	}

	since := time.Now().UTC().Add(-time.Duration(math.Round(hours * float64(time.Hour))))

	if s.chatLog == nil {
		writeJSON(w, http.StatusOK, []chatlog.Record{})
		return
	}

	records, err := s.chatLog.ReadSince(id, since)
	if err != nil {
		if errors.Is(err, chatlog.ErrInvalidID) {
			writeError(w, http.StatusBadRequest, "invalid conversation id")
			return
		}
		writeError(w, http.StatusInternalServerError, "read conversation: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("conversation")
	if !chatlog.ValidConversationID(id) {
		writeError(w, http.StatusBadRequest, "invalid conversation id")
		return
	}
	if s.chatLog == nil {
		writeError(w, http.StatusServiceUnavailable, "chatlog disabled")
		return
	}
	if err := s.chatLog.Remove(id); err != nil {
		writeError(w, http.StatusInternalServerError, "remove conversation: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeHeartbeat(w http.ResponseWriter) error {
	_, err := fmt.Fprint(w, "event: heartbeat\ndata: {}\n\n")
	return err
}

func (s *Server) replayRecentPackets(w http.ResponseWriter, flusher http.Flusher) error {
	if s.receiveLog == nil || s.cfg.ReceiveLog.ReplayWindow <= 0 {
		return nil
	}

	records, err := s.receiveLog.ReadSince(time.Now().UTC().Add(-s.cfg.ReceiveLog.ReplayWindow))
	if err != nil {
		return err
	}

	for _, record := range records {
		if record.ParseError != "" || record.Raw == "" {
			continue
		}

		packet, err := meshcom.ParsePacket([]byte(record.Raw))
		if err != nil {
			continue
		}

		if err := writeSSE(w, events.Event{
			Type: "packet.received",
			Data: map[string]any{
				"remote_addr": record.RemoteAddr,
				"packet":      packet.Packet,
			},
		}); err != nil {
			return err
		}
		flusher.Flush()
	}

	return nil
}

func writeSSE(w http.ResponseWriter, event events.Event) error {
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "encode json", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
