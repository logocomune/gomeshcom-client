// Package stats maintains hourly aggregated counters for packets heard by the
// local MeshCom node. Buckets are cached in memory and persisted to SQLite.
// Entries older than RetentionDays are pruned from memory and storage on every periodic flush.
package stats

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

// Store accumulates per-hour counters in memory and persists them to storage.
type Store struct {
	mu            sync.Mutex
	db            *sql.DB
	retentionDays int
	buckets       map[int64]*Bucket // keyed by truncated-to-hour Unix timestamp
	dirty         bool
}

func NewSQLite(db *sql.DB, cfg Config) *Store {
	return &Store{
		db:            db,
		retentionDays: cfg.RetentionDays,
		buckets:       make(map[int64]*Bucket),
	}
}

// Load reads the stats file into memory, discarding entries outside the
// retention window.
func (s *Store) Load() error {
	return s.loadSQLite(context.Background())
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

	return s.saveSQLite(context.Background(), snap)
}

func (s *Store) loadSQLite(ctx context.Context) error {
	cutoff := s.cutoffUnix(time.Now().UTC())
	rows, err := s.db.QueryContext(ctx, `
		SELECT hour_unix, dm, dm_ack, public, telemetry, position, errors, total
		FROM stats_hourly
		WHERE hour_unix >= ?
	`, cutoff)
	if err != nil {
		return fmt.Errorf("load sqlite stats hourly: %w", err)
	}
	defer rows.Close()

	buckets := make(map[int64]*Bucket)
	for rows.Next() {
		b := &Bucket{}
		if err := rows.Scan(&b.HourUnix, &b.DM, &b.DMAck, &b.Public, &b.Telemetry, &b.Position, &b.Errors, &b.Total); err != nil {
			return fmt.Errorf("scan sqlite stats hourly: %w", err)
		}
		buckets[b.HourUnix] = b
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite stats hourly: %w", err)
	}
	if err := s.loadSQLiteChannels(ctx, buckets); err != nil {
		return err
	}
	if err := s.loadSQLiteDistance(ctx, buckets); err != nil {
		return err
	}

	s.mu.Lock()
	s.buckets = buckets
	s.mu.Unlock()
	return nil
}

func (s *Store) loadSQLiteChannels(ctx context.Context, buckets map[int64]*Bucket) error {
	rows, err := s.db.QueryContext(ctx, `SELECT hour_unix, kind, target, count FROM stats_channels`)
	if err != nil {
		return fmt.Errorf("load sqlite stats channels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var hour int64
		var kind, target string
		var count int
		if err := rows.Scan(&hour, &kind, &target, &count); err != nil {
			return fmt.Errorf("scan sqlite stats channels: %w", err)
		}
		b := buckets[hour]
		if b == nil {
			continue
		}
		if b.Channels == nil {
			b.Channels = make(map[string]int)
		}
		b.Channels[channelKey(kind, target)] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite stats channels: %w", err)
	}
	return nil
}

func (s *Store) loadSQLiteDistance(ctx context.Context, buckets map[int64]*Bucket) error {
	rows, err := s.db.QueryContext(ctx, `SELECT hour_unix, bucket_start_km, bucket_end_km, count FROM stats_distance`)
	if err != nil {
		return fmt.Errorf("load sqlite stats distance: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var hour int64
		var start, end, count int
		if err := rows.Scan(&hour, &start, &end, &count); err != nil {
			return fmt.Errorf("scan sqlite stats distance: %w", err)
		}
		b := buckets[hour]
		if b == nil {
			continue
		}
		if b.DistanceKm == nil {
			b.DistanceKm = make(map[string]int)
		}
		b.DistanceKm[distanceLabel(start, end)] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite stats distance: %w", err)
	}
	return nil
}

func (s *Store) saveSQLite(ctx context.Context, buckets map[int64]*Bucket) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite stats save: %w", err)
	}
	if err := writeSQLiteStats(ctx, tx, buckets); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback sqlite stats save after %w: %w", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite stats save: %w", err)
	}
	return nil
}

func writeSQLiteStats(ctx context.Context, tx *sql.Tx, buckets map[int64]*Bucket) error {
	for _, stmt := range []string{`DELETE FROM stats_distance`, `DELETE FROM stats_channels`, `DELETE FROM stats_hourly`} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("clear sqlite stats: %w", err)
		}
	}
	for _, b := range buckets {
		if err := insertSQLiteBucket(ctx, tx, b); err != nil {
			return err
		}
	}
	return nil
}

func insertSQLiteBucket(ctx context.Context, tx *sql.Tx, b *Bucket) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stats_hourly(hour_unix, dm, dm_ack, public, telemetry, position, errors, total)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, b.HourUnix, b.DM, b.DMAck, b.Public, b.Telemetry, b.Position, b.Errors, b.Total); err != nil {
		return fmt.Errorf("insert sqlite stats hourly %d: %w", b.HourUnix, err)
	}
	for key, count := range b.Channels {
		kind, target := channelParts(key)
		if _, err := tx.ExecContext(ctx, `INSERT INTO stats_channels(hour_unix, kind, target, count) VALUES (?, ?, ?, ?)`, b.HourUnix, kind, target, count); err != nil {
			return fmt.Errorf("insert sqlite stats channel %d/%s: %w", b.HourUnix, key, err)
		}
	}
	for label, count := range b.DistanceKm {
		start, end, err := distanceRange(label)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO stats_distance(hour_unix, bucket_start_km, bucket_end_km, count) VALUES (?, ?, ?, ?)`, b.HourUnix, start, end, count); err != nil {
			return fmt.Errorf("insert sqlite stats distance %d/%s: %w", b.HourUnix, label, err)
		}
	}
	return nil
}

func channelParts(key string) (string, string) {
	switch {
	case key == "broadcast":
		return "broadcast", "*"
	case strings.HasPrefix(key, "ch:"):
		return "channel", strings.TrimPrefix(key, "ch:")
	case strings.HasPrefix(key, "dm:"):
		return "dm", strings.TrimPrefix(key, "dm:")
	default:
		return "channel", key
	}
}

func channelKey(kind, target string) string {
	switch kind {
	case "broadcast":
		return "broadcast"
	case "dm":
		return "dm:" + target
	default:
		return "ch:" + target
	}
}

func distanceRange(label string) (int, int, error) {
	if strings.HasSuffix(label, "+") {
		start, err := strconv.Atoi(strings.TrimSuffix(label, "+"))
		if err != nil {
			return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
		}
		return start, start + int(distanceBinSizeKm), nil
	}
	parts := strings.Split(label, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("parse stats distance label %q", label)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse stats distance label %q: %w", label, err)
	}
	return start, end, nil
}

func distanceLabel(start, end int) string {
	if start >= int(distanceMaxKm) {
		return fmt.Sprintf("%d+", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
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
