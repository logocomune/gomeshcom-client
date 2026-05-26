package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
)

var errUnauthorized = errors.New("unauthorized")

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]time.Time)}
}

func (s *sessionStore) create(ttl time.Duration) (string, time.Time, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", time.Time{}, err
	}

	token := hex.EncodeToString(raw[:])
	expiresAt := time.Now().UTC().Add(ttl)

	s.mu.Lock()
	s.sessions[token] = expiresAt
	s.mu.Unlock()

	return token, expiresAt, nil
}

func (s *sessionStore) valid(token string) bool {
	if token == "" {
		return false
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.sessions[token]
	if !ok {
		return false
	}
	if !expiresAt.After(now) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *sessionStore) delete(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *sessionStore) evictExpired() {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, expiresAt := range s.sessions {
		if !expiresAt.After(now) {
			delete(s.sessions, token)
		}
	}
}

const sessionEvictInterval = 5 * time.Minute

func (s *sessionStore) start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(sessionEvictInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.evictExpired()
			}
		}
	}()
}

func authEnabled(cfg config.Config) bool {
	return cfg.Auth.Username != "" && cfg.Auth.Password != ""
}

func requireAuth(next http.Handler, server *Server) http.Handler {
	if !authEnabled(server.cfg) {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !server.authenticated(r) {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticated(r *http.Request) bool {
	if !authEnabled(s.cfg) {
		return true
	}

	cookie, err := r.Cookie(s.cfg.Auth.CookieName)
	if err != nil {
		return false
	}
	return s.sessions != nil && s.sessions.valid(cookie.Value)
}

func (s *Server) sessionStatus(w http.ResponseWriter, r *http.Request) {
	required := authEnabled(s.cfg)
	authenticated := s.authenticated(r)
	if required && !authenticated {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"required":      true,
			"authenticated": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"required":      required,
		"authenticated": authenticated,
	})
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if !authEnabled(s.cfg) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10) // 1 KB
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if subtle.ConstantTimeCompare([]byte(request.Username), []byte(s.cfg.Auth.Username)) != 1 ||
		subtle.ConstantTimeCompare([]byte(request.Password), []byte(s.cfg.Auth.Password)) != 1 {
		writeUnauthorized(w)
		return
	}

	token, expiresAt, err := s.sessions.create(s.cfg.Auth.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.Auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(s.cfg.Auth.SessionTTL.Seconds()),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if authEnabled(s.cfg) {
		if cookie, err := r.Cookie(s.cfg.Auth.CookieName); err == nil && s.sessions != nil {
			s.sessions.delete(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     s.cfg.Auth.CookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0).UTC(),
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Session realm="gomeshcom"`)
	writeError(w, http.StatusUnauthorized, "unauthorized")
}
