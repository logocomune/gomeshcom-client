package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
)

var errUnauthorized = errors.New("unauthorized")

type persistedSessions struct {
	Sessions map[string]time.Time `json:"sessions"`
}

type sessionStore struct {
	mu       sync.Mutex
	path     string
	sessions map[string]time.Time
}

func newSessionStore(path string) *sessionStore {
	store := &sessionStore{
		path:     path,
		sessions: make(map[string]time.Time),
	}
	store.load()
	return store
}

func (s *sessionStore) load() {
	if s.path == "" {
		return
	}

	file, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer file.Close()

	var persisted persistedSessions
	if err := json.NewDecoder(file).Decode(&persisted); err != nil {
		return
	}

	now := time.Now().UTC()
	for tokenHash, expiresAt := range persisted.Sessions {
		if expiresAt.After(now) {
			s.sessions[tokenHash] = expiresAt
		}
	}
}

func (s *sessionStore) create(ttl time.Duration) (string, time.Time, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", time.Time{}, err
	}

	token := hex.EncodeToString(raw[:])
	expiresAt := time.Now().UTC().Add(ttl)
	hash := hashSessionToken(token)

	s.mu.Lock()
	s.sessions[hash] = expiresAt
	if err := s.persistLocked(); err != nil {
		delete(s.sessions, hash)
		s.mu.Unlock()
		return "", time.Time{}, err
	}
	s.mu.Unlock()

	return token, expiresAt, nil
}

func (s *sessionStore) valid(token string) bool {
	if token == "" {
		return false
	}

	now := time.Now().UTC()
	hash := hashSessionToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.sessions[hash]
	if !ok {
		return false
	}
	if !expiresAt.After(now) {
		delete(s.sessions, hash)
		_ = s.persistLocked()
		return false
	}
	return true
}

func (s *sessionStore) delete(token string) error {
	if token == "" {
		return nil
	}

	hash := hashSessionToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, exists := s.sessions[hash]
	if !exists {
		return nil
	}
	delete(s.sessions, hash)
	if err := s.persistLocked(); err != nil {
		s.sessions[hash] = expiresAt
		return err
	}
	return nil
}

func (s *sessionStore) evictExpired() {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for tokenHash, expiresAt := range s.sessions {
		if !expiresAt.After(now) {
			delete(s.sessions, tokenHash)
			changed = true
		}
	}
	if changed {
		_ = s.persistLocked()
	}
}

func (s *sessionStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	tmpPath := s.path + ".tmp"
	temp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary session file: %w", err)
	}

	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tmpPath)
	}
	encoder := json.NewEncoder(temp)
	if err := encoder.Encode(persistedSessions{Sessions: s.sessions}); err != nil {
		cleanup()
		return fmt.Errorf("encode sessions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync sessions: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close sessions: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace sessions: %w", err)
	}
	return nil
}

func sessionPersistencePath(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "http-sessions.json")
}

func hashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
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
			if err := s.sessions.delete(cookie.Value); err != nil {
				writeError(w, http.StatusInternalServerError, "delete session")
				return
			}
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
