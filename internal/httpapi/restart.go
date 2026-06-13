package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// shutdown handles POST /api/shutdown.
//
// It responds immediately so the client can receive the confirmation before the
// process shuts down, then invokes the registered shutdown callback after a short
// delay to allow the response to be flushed.
//
// Returns 501 when no shutdown callback has been registered via WithShutdownFunc.
func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	if s.shutdownFunc == nil {
		http.Error(w, "shutdown not configured", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		s.shutdownFunc()
	}()
}

// restart handles POST /api/restart.
//
// It responds immediately so the client can receive the confirmation before the
// process shuts down, then invokes the registered restart callback after a short
// delay to allow the response to be flushed.
//
// Returns 501 when no restart callback has been registered via WithRestartFunc.
func (s *Server) restart(w http.ResponseWriter, r *http.Request) {
	if s.restartFunc == nil {
		http.Error(w, "restart not configured", http.StatusNotImplemented)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		// Give the HTTP layer time to flush the response to the client before
		// the server is shut down.
		time.Sleep(500 * time.Millisecond)
		s.restartFunc()
	}()
}
