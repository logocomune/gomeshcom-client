// Package dmstats tracks per-callsign DM send and acknowledgement counters.
// For each destination callsign, it records how many DMs were sent and how
// many acks were received. When the destination has an SSID suffix, the
// counter is split into a full entry (e.g. "CALL-1") and a base entry
// (e.g. "CALL"). Latency (ms from send to ack) is accumulated only on the
// full entry; the base entry tracks only sent/ack counts.
// Counters are cumulative (not bucketed by time) and persisted to SQLite.
package dmstats

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
)

const saveInterval = time.Minute

// DefaultPath returns the conventional dm stats file path under the given data root.
func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "stats", "dm_stats.json")
}

// Entry holds cumulative DM counters for a single callsign.
type Entry struct {
	Sent int `json:"sent"`
	Ack  int `json:"ack"`
}

// Store accumulates per-callsign DM counters in memory and persists them to
// a single JSON file.
type Store struct {
	mu      sync.Mutex
	db      *sql.DB
	entries map[string]*Entry // keyed by uppercase callsign (full or base)
	dirty   bool
}

func NewSQLite(db *sql.DB) *Store {
	return &Store{
		db:      db,
		entries: make(map[string]*Entry),
	}
}

// Load reads the SQLite store into memory.
func (s *Store) Load() error {
	return s.loadSQLite()
}

// Start runs the periodic flush loop until ctx is cancelled, then performs a
// final flush.
func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.save(); err != nil {
				slog.Error("dmstats: final flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := s.save(); err != nil {
				slog.Error("dmstats: flush failed", "error", err)
			}
		}
	}
}

// RecordSent increments the sent counter for destination and, when destination
// carries a numeric SSID, also for its base callsign.
func (s *Store) RecordSent(destination string) {
	full, base := callsignPair(destination)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.getOrCreate(full).Sent++
	if base != full {
		s.getOrCreate(base).Sent++
	}
	s.dirty = true
}

// RecordAck increments the ack counter for destination and, when destination
// carries a numeric SSID, also for its base callsign.
func (s *Store) RecordAck(destination string) {
	full, base := callsignPair(destination)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.getOrCreate(full).Ack++
	if base != full {
		s.getOrCreate(base).Ack++
	}
	s.dirty = true
}

// Snapshot returns a copy of all entries keyed by callsign.
func (s *Store) Snapshot() map[string]Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]Entry, len(s.entries))
	for k, v := range s.entries {
		out[k] = *v
	}
	return out
}

// ---- internal helpers -------------------------------------------------------

// callsignPair returns (full, base) for a destination. full is uppercased; base
// is the callsign without its numeric SSID suffix. When there is no SSID,
// full == base.
func callsignPair(destination string) (full, base string) {
	full = strings.ToUpper(strings.TrimSpace(destination))
	base = chatlog.BaseCall(full)
	return full, base
}

func (s *Store) getOrCreate(key string) *Entry {
	e, ok := s.entries[key]
	if !ok {
		e = &Entry{}
		s.entries[key] = e
	}
	return e
}

func (s *Store) save() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	snap := make(map[string]*Entry, len(s.entries))
	for k, v := range s.entries {
		cp := *v
		snap[k] = &cp
	}
	s.dirty = false
	s.mu.Unlock()

	return writeSQLite(s.db, snap)
}

func (s *Store) loadSQLite() error {
	rows, err := s.db.Query(`SELECT callsign, sent, ack FROM dm_stats`)
	if err != nil {
		return fmt.Errorf("query dm stats: %w", err)
	}
	defer rows.Close()

	entries := make(map[string]*Entry)
	for rows.Next() {
		var callsign string
		entry := &Entry{}
		if err := rows.Scan(&callsign, &entry.Sent, &entry.Ack); err != nil {
			return fmt.Errorf("scan dm stats: %w", err)
		}
		entries[callsign] = entry
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dm stats: %w", err)
	}

	s.mu.Lock()
	s.entries = entries
	s.dirty = false
	s.mu.Unlock()
	return nil
}

func writeSQLite(db *sql.DB, entries map[string]*Entry) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin dm stats save: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM dm_stats`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear dm stats: %w", err)
	}
	for callsign, entry := range entries {
		_, err := tx.Exec(`INSERT INTO dm_stats(callsign, sent, ack) VALUES (?, ?, ?)`, callsign, entry.Sent, entry.Ack)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save dm stats %s: %w", callsign, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dm stats save: %w", err)
	}
	return nil
}
