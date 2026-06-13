// Package dmstats tracks per-callsign DM send and acknowledgement counters.
// For each destination callsign, it records how many DMs were sent and how
// many acks were received. When the destination has an SSID suffix, the
// counter is split into a full entry (e.g. "CALL-1") and a base entry
// (e.g. "CALL"). Latency (ms from send to ack) is accumulated only on the
// full entry; the base entry tracks only sent/ack counts.
// Counters are cumulative (not bucketed by time) and persisted to a JSON file.
package dmstats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	path    string
	entries map[string]*Entry // keyed by uppercase callsign (full or base)
	dirty   bool
}

// New constructs a Store. Call Load to restore persisted data, then Start to
// begin periodic flushing.
func New(path string) *Store {
	return &Store{
		path:    path,
		entries: make(map[string]*Entry),
	}
}

// Load reads the store file into memory. A missing file is not an error.
func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dm stats file: %w", err)
	}

	var raw map[string]*Entry
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal dm stats file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range raw {
		s.entries[k] = v
	}
	return nil
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

	return writeFileAtomically(s.path, snap)
}

func writeFileAtomically(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dmstats dir: %w", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal dmstats: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write dmstats tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename dmstats file: %w", err)
	}
	return nil
}
