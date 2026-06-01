// Package stats maintains hourly aggregated counters for packets heard by the
// local MeshCom node. All buckets are stored in a single JSON file keyed by
// UTC hour (Unix timestamp). Entries older than RetentionDays are pruned from
// memory and disk on every periodic flush.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const SaveInterval = time.Minute

// Bucket holds aggregated packet counts for a single UTC hour.
type Bucket struct {
	HourUnix   int64          `json:"hour"`
	DM         int            `json:"dm"`
	DMAck      int            `json:"dm_ack"`
	Public     int            `json:"public"`
	Telemetry  int            `json:"telemetry"`
	Position   int            `json:"position"`
	Errors     int            `json:"errors"`
	Total      int            `json:"total"`
	DistanceKm map[string]int `json:"distance_km,omitempty"`
	// Channels holds per-channel/per-DM message counts.
	// Keys: "broadcast", "ch:<N>", "dm:<CALLSIGN>".
	// DM key counts both inbound DMs and delivered acks combined.
	Channels map[string]int `json:"channels,omitempty"`
}

// Kind identifies the type of a recorded packet.
type Kind string

const (
	KindDM        Kind = "dm"
	KindPublic    Kind = "public"
	KindTelemetry Kind = "telemetry"
	KindPosition  Kind = "position"
	KindError     Kind = "error"
)

// Config holds the configuration for the stats store.
type Config struct {
	Enabled       bool
	Path          string // e.g. data/stats/stats.json
	RetentionDays int
}

// DefaultPath returns the conventional stats file path under the given data root.
func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "stats", "stats.json")
}

// Store accumulates per-hour counters in memory and persists them to a single
// JSON file.
type Store struct {
	mu            sync.Mutex
	path          string
	retentionDays int
	buckets       map[int64]*Bucket // keyed by truncated-to-hour Unix timestamp
	dirty         bool
}

// New constructs a Store. Call Load to restore persisted buckets, then Start to
// begin periodic flushing.
func New(cfg Config) *Store {
	return &Store{
		path:          cfg.Path,
		retentionDays: cfg.RetentionDays,
		buckets:       make(map[int64]*Bucket),
	}
}

// Load reads the stats file into memory, discarding entries outside the
// retention window.
func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read stats file: %w", err)
	}

	var raw map[int64]*Bucket
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal stats file: %w", err)
	}

	cutoff := s.cutoffUnix(time.Now().UTC())

	s.mu.Lock()
	defer s.mu.Unlock()

	for k, b := range raw {
		if k >= cutoff {
			s.buckets[k] = b
		}
	}
	return nil
}

// Start runs the periodic flush+prune loop until ctx is cancelled, then
// performs a final flush.
func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.save(); err != nil {
				slog.Error("stats: final flush failed", "error", err)
			}
			return
		case <-ticker.C:
			s.pruneExpired(time.Now().UTC())
			if err := s.save(); err != nil {
				slog.Error("stats: flush failed", "error", err)
			}
		}
	}
}

// RecordPacket increments the counter for the given kind at the given timestamp.
// When distanceKm is non-nil (only meaningful for position packets), the value
// is added to the DistanceKm histogram.
func (s *Store) RecordPacket(kind Kind, receivedAt time.Time, distanceKm *float64) {
	hour := hourKey(receivedAt)

	s.mu.Lock()
	defer s.mu.Unlock()

	b := s.getOrCreate(hour)
	switch kind {
	case KindDM:
		b.DM++
	case KindPublic:
		b.Public++
	case KindTelemetry:
		b.Telemetry++
	case KindPosition:
		b.Position++
		if distanceKm != nil {
			label := DistanceBucketLabel(*distanceKm)
			if b.DistanceKm == nil {
				b.DistanceKm = make(map[string]int)
			}
			b.DistanceKm[label]++
		}
	case KindError:
		b.Errors++
		s.dirty = true
		return // errors excluded from Total
	}
	b.Total++
	s.dirty = true
}

// RecordDMAck increments the DMAck counter for the hour of receivedAt.
func (s *Store) RecordDMAck(receivedAt time.Time) {
	hour := hourKey(receivedAt)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.getOrCreate(hour).DMAck++
	s.dirty = true
}

// RecordChannel increments the per-channel counter for key in the hour of receivedAt.
// key format: "broadcast", "ch:<N>", "dm:<CALLSIGN>".
func (s *Store) RecordChannel(key string, receivedAt time.Time) {
	if key == "" {
		return
	}
	hour := hourKey(receivedAt)

	s.mu.Lock()
	defer s.mu.Unlock()

	b := s.getOrCreate(hour)
	if b.Channels == nil {
		b.Channels = make(map[string]int)
	}
	b.Channels[key]++
	s.dirty = true
}

// ReadRange returns all buckets whose hour falls in [from, to), sorted by hour.
func (s *Store) ReadRange(from, to time.Time) ([]Bucket, error) {
	fromKey := hourKey(from)
	toKey := hourKey(to)

	s.mu.Lock()
	defer s.mu.Unlock()

	var result []Bucket
	for k, b := range s.buckets {
		if k >= fromKey && k <= toKey {
			result = append(result, *b)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].HourUnix < result[j].HourUnix
	})
	return result, nil
}

// ---- internal helpers -------------------------------------------------------

func (s *Store) getOrCreate(hourUnix int64) *Bucket {
	b, ok := s.buckets[hourUnix]
	if !ok {
		b = &Bucket{HourUnix: hourUnix}
		s.buckets[hourUnix] = b
	}
	return b
}

func (s *Store) pruneExpired(now time.Time) {
	cutoff := s.cutoffUnix(now)

	s.mu.Lock()
	defer s.mu.Unlock()

	for k := range s.buckets {
		if k < cutoff {
			delete(s.buckets, k)
			s.dirty = true
		}
	}
}

func (s *Store) save() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	// Snapshot under lock, write outside.
	snap := make(map[int64]*Bucket, len(s.buckets))
	for k, v := range s.buckets {
		cp := *v
		snap[k] = &cp
	}
	s.dirty = false
	s.mu.Unlock()

	return writeFileAtomically(s.path, snap)
}

func (s *Store) cutoffUnix(now time.Time) int64 {
	if s.retentionDays <= 0 {
		return 0 // keep everything
	}
	return now.UTC().Add(-time.Duration(s.retentionDays) * 24 * time.Hour).Unix()
}

func hourKey(t time.Time) int64 {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), 0, 0, 0, time.UTC).Unix()
}

func writeFileAtomically(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create stats dir: %w", err)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write stats tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename stats file: %w", err)
	}
	return nil
}
