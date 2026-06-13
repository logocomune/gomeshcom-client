// Package station manages the runtime station identity (local callsign).
//
// Identity holds the active callsign behind a read-write mutex so that all
// long-lived services can read the latest value without a process restart.
// Persistence mirrors the channelshow.Store pattern: a JSON file under
// data/configs/ is updated on a background ticker and flushed on shutdown.
//
// Precedence at startup: persisted station.json > GOMESHCOM_MY_CALL config
// default. Runtime updates via Update override both.
package station

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/callsign"
)

const saveInterval = time.Minute

type Identity struct {
	mu      sync.RWMutex
	current string
	path    string // empty → in-memory only, no persistence
	dirty   bool
}

type persistedIdentity struct {
	Callsign string `json:"callsign"`
}

// DefaultPath returns the canonical path for the persisted station config.
func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "configs", "station.json")
}

// New loads persisted callsign from path (if present and valid). Falls back to
// fallback (the startup config value). Returns an error only for I/O failures.
func New(path, fallback string) (*Identity, error) {
	id := &Identity{
		path:    path,
		current: callsign.Normalize(fallback),
	}

	if err := id.load(); err != nil {
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
	if id.path == "" {
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
	if id.path == "" {
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

	if err := writeFileAtomically(id.path, snapshot); err != nil {
		id.mu.Lock()
		id.dirty = true
		id.mu.Unlock()
		return err
	}

	return nil
}

// load reads the persisted file and replaces the current callsign if the file
// contains a valid callsign. A missing file is treated as "not yet persisted"
// and is not an error.
func (id *Identity) load() error {
	if id.path == "" {
		return nil
	}

	_ = os.Remove(id.path + ".tmp")

	file, err := os.Open(id.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open station identity: %w", err)
	}
	defer file.Close()

	var p persistedIdentity
	if err := json.NewDecoder(file).Decode(&p); err != nil {
		return fmt.Errorf("decode station identity: %w", err)
	}

	normalized := callsign.Normalize(p.Callsign)
	if !callsign.IsValid(normalized) {
		slog.Warn("station identity file contains invalid callsign — using config default", "value", p.Callsign)
		return nil
	}

	id.current = normalized
	return nil
}

func writeFileAtomically(path, cs string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create station identity dir: %w", err)
	}

	tmpPath := path + ".tmp"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp station identity file: %w", err)
	}

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(persistedIdentity{Callsign: cs}); encErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode station identity: %w", encErr)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp station identity file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp station identity file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace station identity file: %w", err)
	}

	return nil
}
