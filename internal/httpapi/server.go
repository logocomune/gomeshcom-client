package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/outbox"
	"github.com/logocomune/gomeshcom-client/internal/positions"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
	"github.com/logocomune/gomeshcom-client/internal/sendcache"
	"github.com/logocomune/gomeshcom-client/internal/udpbridge"
	"github.com/logocomune/gomeshcom-client/internal/webui"
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
	outbox     *outbox.Outbox
	sessions   *sessionStore
	cancel     context.CancelFunc
}

const outgoingEchoTimeout = 5 * time.Second

func NewServer(cfg config.Config, version string, bus *events.Bus, positionStore *positions.Store, receiveLog *receivelog.Logger, chatLog *chatlog.Logger, bridge messageSender, sc *sendcache.Cache) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		cfg:        cfg,
		version:    version,
		bus:        bus,
		positions:  positionStore,
		receiveLog: receiveLog,
		chatLog:    chatLog,
		bridge:     bridge,
		sendCache:  sc,
		cancel:     cancel,
	}
	server.outbox = outbox.New(outgoingEchoTimeout, server.handleOutgoingTimeout)
	server.watchOutgoingEchoes(ctx)
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
	handler := cacheHeadersMiddleware(mux)
	if s.cfg.RequestLog.Enabled {
		return requestLogMiddleware(handler)
	}
	return handler
}

func cacheHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCacheHeaders(w.Header(), r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func setCacheHeaders(header http.Header, path string) {
	switch {
	case strings.HasPrefix(path, "/api/"):
		setNoCacheHeaders(header)
	case strings.HasPrefix(path, "/_app/immutable/"):
		setImmutableCacheHeaders(header)
	case path == "/" || path == "/index.html":
		setIndexNoCacheHeaders(header)
	}
}

func setNoCacheHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

func setImmutableCacheHeaders(header http.Header) {
	header.Set("Cache-Control", "public, max-age=31536000, immutable")
}

func setIndexNoCacheHeaders(header http.Header) {
	header.Set("Cache-Control", "no-cache, must-revalidate")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

type flushStatusRecorder struct {
	*statusRecorder
}

func (r *flushStatusRecorder) Flush() {
	r.ResponseWriter.(http.Flusher).Flush()
}

func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now().UTC()
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		responseWriter := http.ResponseWriter(recorder)
		if _, ok := w.(http.Flusher); ok {
			responseWriter = &flushStatusRecorder{statusRecorder: recorder}
		}

		next.ServeHTTP(responseWriter, r)

		duration := time.Since(startedAt)
		slog.Info("http request",
			"method", r.Method,
			"endpoint", r.URL.Path,
			"status", recorder.statusCode,
			"caller_ip", callerIP(r),
			"started_at", startedAt.Format(time.RFC3339Nano),
			"duration", duration.String(),
			"duration_ms", float64(duration.Microseconds())/1000,
		)
	})
}

func callerIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Rela-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value != "" {
			return value
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
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
	if s.outbox != nil && !s.cfg.Send.DisableTx {
		s.outbox.Register(s.cfg.MyCall, outgoing.Destination, outgoing.Message, time.Now().UTC())
	}

	writeJSON(w, http.StatusAccepted, outgoing)
}

func (s *Server) watchOutgoingEchoes(ctx context.Context) {
	if s.bus == nil {
		return
	}
	subscriber := s.bus.Subscribe(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-subscriber:
				if !ok {
					return
				}
				if event.Type != "packet.received" {
					continue
				}
				message, ok := textMessageFromEvent(event)
				if !ok {
					continue
				}
				if s.outbox != nil {
					s.outbox.Confirm(message.Source, message.Destination, message.Message)
				}
			}
		}
	}()
}

func textMessageFromEvent(event events.Event) (meshcom.TextMessage, bool) {
	data, ok := event.Data.(map[string]any)
	if !ok {
		return meshcom.TextMessage{}, false
	}
	message, ok := data["packet"].(meshcom.TextMessage)
	return message, ok
}

func (s *Server) handleOutgoingTimeout(message outbox.PendingMessage) {
	record := chatlog.Record{
		ReceivedAt:     time.Now().UTC(),
		Src:            message.Source,
		Dst:            message.Destination,
		Msg:            message.Message,
		Direction:      "outbound",
		DeliveryStatus: "failed",
	}
	if s.chatLog != nil {
		var err error
		record, err = s.chatLog.AppendFailed(message.Source, message.Destination, message.Message, record.ReceivedAt)
		if err != nil {
			slog.Error("failed outgoing chat log write failed", "error", err)
			return
		}
	}
	if s.bus != nil {
		s.bus.Publish(events.Event{Type: "message.failed", Data: record})
	}
}

func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	replayCutoff, replayEnabled, err := s.replayCutoff(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	setNoCacheHeaders(w.Header())
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

	if err := s.replayRecentPackets(w, flusher, replayCutoff, replayEnabled); err != nil {
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

	defaultHours := s.defaultChatHistoryHours(id)
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

func (s *Server) defaultChatHistoryHours(conversationID string) float64 {
	if strings.HasPrefix(conversationID, "DM_") {
		return (30 * 24 * time.Hour).Hours()
	}
	return s.cfg.ChatLog.HistoryWindow.Hours()
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

func (s *Server) replayCutoff(r *http.Request) (time.Time, bool, error) {
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	var cutoff time.Time
	var hasCutoff bool
	if from != "" {
		parsed, err := time.Parse(time.RFC3339Nano, from)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid from timestamp")
		}
		cutoff = parsed.UTC()
		hasCutoff = true
	}

	if s.cfg.ReceiveLog.ReplayWindow > 0 {
		minCutoff := time.Now().UTC().Add(-s.cfg.ReceiveLog.ReplayWindow)
		if !hasCutoff || cutoff.Before(minCutoff) {
			cutoff = minCutoff
		}
		return cutoff, true, nil
	}

	if hasCutoff {
		return cutoff, true, nil
	}
	return time.Time{}, false, nil
}

func writeHeartbeat(w http.ResponseWriter) error {
	_, err := fmt.Fprint(w, "event: heartbeat\ndata: {}\n\n")
	return err
}

func (s *Server) replayRecentPackets(w http.ResponseWriter, flusher http.Flusher, cutoff time.Time, enabled bool) error {
	if s.receiveLog == nil || !enabled {
		return nil
	}

	records, err := s.receiveLog.ReadSince(cutoff)
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
				"received_at": record.ReceivedAt.UTC().Format(time.RFC3339Nano),
				"replay":      true,
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

func (s *Server) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}
