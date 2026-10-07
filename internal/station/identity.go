// Package station manages the runtime station identity (local callsign).
//
// Identity holds the active callsign behind a read-write mutex so that all
// long-lived services can read the latest value without a process restart.
// Runtime persistence uses SQLite.
package station

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/callsign"
)

const saveInterval = time.Minute

type Identity struct {
	mu      sync.RWMutex
	current string
	db      *sql.DB
	dirty   bool
}

func NewSQLite(db *sql.DB, fallback string) (*Identity, error) {
	id := &Identity{
		db:      db,
		current: callsign.Normalize(fallback),
	}
	if err := id.loadSQLite(context.Background()); err != nil {
		return nil, err
	}
	return id, nil
}

// NewInMemory creates a non-persistent Identity useful for testing.
// The callsign is normalized; an invalid value is stored as-is (callers must
// ensure they pass valid callsigns).
func NewInMemory(cs string) *Identity {
	return &Identity{
		current: callsign.Normalize(cs),
	}
}

// Current returns the active callsign. Concurrency-safe.
func (id *Identity) Current() string {
	id.mu.RLock()
	defer id.mu.RUnlock()
	return id.current
}

// Update normalizes and validates cs. On success, the new value becomes
// current and the state is marked dirty for persistence. Returns the accepted
// callsign. Returns an error (and keeps the previous value) if cs is invalid.
func (id *Identity) Update(cs string) (string, error) {
	normalized := callsign.Normalize(cs)
	if !callsign.IsValid(normalized) {
		return "", fmt.Errorf("invalid callsign %q: must be 3-10 alphanumeric characters with an optional numeric SSID (e.g. IU5PMP-1)", cs)
	}

	id.mu.Lock()
	defer id.mu.Unlock()

	if id.current == normalized {
		return normalized, nil // no-op
	}

	id.current = normalized
	id.dirty = true
	return normalized, nil
}

// Start runs the periodic flush loop until ctx is cancelled, then performs a
// final flush. Call in a dedicated goroutine.
func (id *Identity) Start(ctx context.Context) {
	if id.db == nil {
		return // in-memory only
	}

	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := id.SaveIfDirty(); err != nil {
				slog.Error("station identity flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := id.SaveIfDirty(); err != nil {
				slog.Error("station identity save failed", "error", err)
			}
		}
	}
}

// SaveIfDirty persists the current callsign if it has changed since the last
// save. No-op if the Identity is in-memory only or nothing has changed.
func (id *Identity) SaveIfDirty() error {
	if id.db == nil {
		return nil
	}

	id.mu.Lock()
	if !id.dirty {
		id.mu.Unlock()
		return nil
	}
	snapshot := id.current
	id.dirty = false
	id.mu.Unlock()

	if err := id.write(snapshot); err != nil {
		id.mu.Lock()
		id.dirty = true
		id.mu.Unlock()
		return err
	}

	return nil
}

func (id *Identity) write(snapshot string) error {
	if _, err := id.db.ExecContext(context.Background(), `
		INSERT INTO station_identity(id, callsign)
		VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET callsign = excluded.callsign
	`, snapshot); err != nil {
		return fmt.Errorf("save station identity sqlite: %w", err)
	}
	return nil
}

func (id *Identity) loadSQLite(ctx context.Context) error {
	var persisted string
	err := id.db.QueryRowContext(ctx, `SELECT callsign FROM station_identity WHERE id = 1`).Scan(&persisted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("load station identity sqlite: %w", err)
	}

	normalized := callsign.Normalize(persisted)
	if !callsign.IsValid(normalized) {
		slog.Warn("station identity row contains invalid callsign; using config default", "value", persisted)
		return nil
	}

	id.current = normalized
	return nil
}
