package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
)

var errUnauthorized = errors.New("unauthorized")

type sessionStore struct {
	mu       sync.Mutex
	db       *sql.DB
	sessions map[string]time.Time
}

func newMemorySessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]time.Time)}
}

func newSQLiteSessionStore(db *sql.DB) *sessionStore {
	store := &sessionStore{
		db:       db,
		sessions: make(map[string]time.Time),
	}
	store.load()
	return store
}

func (s *sessionStore) load() {
	if s.db != nil {
		s.loadSQLite(context.Background())
	}
}

func (s *sessionStore) loadSQLite(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT token_hash, expires_at FROM http_sessions`)
	if err != nil {
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	for rows.Next() {
		var tokenHash string
		var expiresRaw string
		if err := rows.Scan(&tokenHash, &expiresRaw); err != nil {
			return
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, expiresRaw)
		if err != nil {
			continue
		}
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
	if s.db == nil {
		return nil
	}
	return s.persistSQLiteLocked(context.Background())
}

func (s *sessionStore) persistSQLiteLocked(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM http_sessions`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear sqlite sessions: %w", err)
	}
	for tokenHash, expiresAt := range s.sessions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO http_sessions(token_hash, expires_at) VALUES (?, ?)`, tokenHash, expiresAt.UTC().Format(time.RFC3339Nano)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert sqlite session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite sessions: %w", err)
	}
	return nil
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
