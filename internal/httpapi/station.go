package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

type myCallRequest struct {
	Callsign string `json:"callsign"`
}

// getMyCall handles GET /api/adm/configs/my-call.
// Returns the active callsign and current station identity context.
func (s *Server) getMyCall(w http.ResponseWriter, _ *http.Request) {
	forwardTargets, _ := config.ParseForwardTargets(s.cfg.Forward.Targets)
	writeJSON(w, http.StatusOK, stationIdentityEvent{
		Callsign:           s.identity.Current(),
		Version:            s.version,
		TxDisabled:         s.cfg.DemoMode,
		ForwardTargetCount: len(forwardTargets),
	})
}

// updateMyCall handles PUT /api/adm/configs/my-call.
// Validates, persists, and broadcasts the new callsign via SSE.
func (s *Server) updateMyCall(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<13) // 8 KB

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	var req myCallRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	accepted, err := s.identity.Update(req.Callsign)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if saveErr := s.identity.SaveIfDirty(); saveErr != nil {
		slog.Error("station identity save failed", "error", saveErr)
		writeError(w, http.StatusInternalServerError, "persist callsign failed")
		return
	}

	forwardTargets, _ := config.ParseForwardTargets(s.cfg.Forward.Targets)
	updated := stationIdentityEvent{
		Callsign:           accepted,
		Version:            s.version,
		TxDisabled:         s.cfg.DemoMode,
		ForwardTargetCount: len(forwardTargets),
	}

	if s.bus != nil {
		s.bus.Publish(events.Event{Type: "station.identity", Data: updated})
	}

	writeJSON(w, http.StatusOK, updated)
}
