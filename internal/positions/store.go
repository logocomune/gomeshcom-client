package positions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

const SaveInterval = time.Minute

type Record struct {
	Latitude       float64    `json:"lat"`
	Longitude      float64    `json:"lng"`
	Altitude       int        `json:"alt"`
	HardwareID     string     `json:"hw_id,omitempty"`
	FirstSeen      time.Time  `json:"firstseen"`
	LastSeen       time.Time  `json:"lastseen"`
	LastDirectSeen *time.Time `json:"lastdirectseen,omitempty"`
	RSSI           int        `json:"rssi"`
	SNR            int        `json:"snr"`
	Via            []string   `json:"via"`
}

type Store struct {
	mu         sync.Mutex
	db         *sql.DB
	records    map[string]Record
	dirtyNodes map[string]bool
	dirty      bool
}

func NewSQLite(db *sql.DB) *Store {
	return &Store{
		db:         db,
		records:    make(map[string]Record),
		dirtyNodes: make(map[string]bool),
	}
}

func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "nodes", "positions.json")
}

func (s *Store) Load() error {
	return s.loadSQLite()
}

func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("position store flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("position store save failed", "error", err)
			}
		}
	}
}

// Update writes coord/metadata for the origin of a pos packet and applies
// freshness rules: direct packets update rssi/snr/lastDirectSeen on origin;
// indirect packets update only lastSeen on origin and update lastDirectSeen/rssi/snr
// on the last relay (if a record for it already exists).
func (s *Store) Update(position meshcom.Position, seenAt time.Time) bool {
	callsign, via := meshcom.SplitSourcePath(position.Source)
	if callsign == "" {
		return false
	}

	isDirect := len(via) == 0

	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.records[callsign]

	record := Record{
		Latitude:   position.Latitude,
		Longitude:  position.Longitude,
		Altitude:   position.Altitude,
		HardwareID: string(position.HardwareID),
		FirstSeen:  seenAt.UTC(),
		Via:        via,
		// Freshness fields: preserve existing until applyFreshness runs.
		LastSeen:       seenAt.UTC(),
		LastDirectSeen: nil,
		RSSI:           0,
		SNR:            0,
	}
	if exists {
		record.FirstSeen = current.FirstSeen
		record.LastDirectSeen = current.LastDirectSeen
		record.RSSI = current.RSSI
		record.SNR = current.SNR
	}

	if isDirect {
		applyFreshness(&record, freshnessDirect, seenAt, position.RSSI, position.SNR)
	} else {
		applyFreshness(&record, freshnessIndirect, seenAt, nil, nil)
		s.touchViaChainLocked(via, position.RSSI, position.SNR, seenAt)
	}

	if exists && reflect.DeepEqual(current, record) {
		return false
	}

	s.setRecordLocked(callsign, record)
	return true
}

// TouchFromPacket updates freshness for msg/tele packets without changing
// coordinates. Only updates records that already exist — never creates new ones.
// Direct packets: full freshness (lastSeen, lastDirectSeen, rssi, snr) on origin.
// Indirect packets: lastSeen only on origin; full freshness on last relay.
func (s *Store) TouchFromPacket(src string, rssi, snr *int, seenAt time.Time) bool {
	callsign, via := meshcom.SplitSourcePath(src)
	if callsign == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	isDirect := len(via) == 0

	if isDirect {
		if rec, exists := s.records[callsign]; exists {
			applyFreshness(&rec, freshnessDirect, seenAt, rssi, snr)
			s.setRecordLocked(callsign, rec)
			changed = true
		}
	} else {
		if rec, exists := s.records[callsign]; exists {
			applyFreshness(&rec, freshnessIndirect, seenAt, nil, nil)
			s.setRecordLocked(callsign, rec)
			changed = true
		}
		changed = s.touchViaChainLocked(via, rssi, snr, seenAt) || changed
	}
	return changed
}

// Get returns the position record for the given callsign (case-sensitive, must
// match the key stored during Update — typically upper-case). Returns false
// when the callsign is not known.
func (s *Store) Get(callsign string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[callsign]
	return r, ok
}

func (s *Store) Snapshot() map[string]Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make(map[string]Record, len(s.records))
	for callsign, record := range s.records {
		records[callsign] = record
	}
	return records
}

func (s *Store) SaveIfDirty() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	records := make(map[string]Record, len(s.dirtyNodes))
	for callsign := range s.dirtyNodes {
		records[callsign] = s.records[callsign]
	}
	dirtyNodes := s.dirtyNodeSet()
	s.dirty = false
	s.dirtyNodes = make(map[string]bool)
	s.mu.Unlock()

	if err := writeSQLite(s.db, records); err != nil {
		s.mu.Lock()
		s.dirty = true
		for callsign := range dirtyNodes {
			s.dirtyNodes[callsign] = true
		}
		s.mu.Unlock()
		return err
	}

	return nil
}

func (s *Store) setRecordLocked(callsign string, record Record) {
	s.records[callsign] = record
	s.markDirtyLocked(callsign)
}

func (s *Store) markDirtyLocked(callsign string) {
	s.dirtyNodes[callsign] = true
	s.dirty = true
}

func (s *Store) dirtyNodeSet() map[string]bool {
	dirtyNodes := make(map[string]bool, len(s.dirtyNodes))
	for callsign := range s.dirtyNodes {
		dirtyNodes[callsign] = true
	}
	return dirtyNodes
}

type freshnessMode int

const (
	freshnessIndirect freshnessMode = iota
	freshnessDirect
)

// applyFreshness updates lastSeen and, for direct mode, lastDirectSeen/rssi/snr.
// For indirect mode only lastSeen is updated; rssi/snr/lastDirectSeen are left as-is.
func applyFreshness(rec *Record, mode freshnessMode, seenAt time.Time, rssi, snr *int) {
	rec.LastSeen = seenAt.UTC()
	if mode == freshnessDirect {
		t := seenAt.UTC()
		rec.LastDirectSeen = &t
		if rssi != nil {
			rec.RSSI = *rssi
		}
		if snr != nil {
			rec.SNR = *snr
		}
	}
}

// touchLastHopLocked applies direct freshness to an existing record.
// Caller must hold s.mu.
func (s *Store) touchLastHopLocked(callsign string, rssi, snr *int, seenAt time.Time) bool {
	rec, exists := s.records[callsign]
	if !exists {
		return false
	}
	applyFreshness(&rec, freshnessDirect, seenAt, rssi, snr)
	s.setRecordLocked(callsign, rec)
	return true
}

// touchViaChainLocked applies indirect freshness to every relay in via, then
// applies direct freshness to last hop. Caller must hold s.mu.
func (s *Store) touchViaChainLocked(via []string, rssi, snr *int, seenAt time.Time) bool {
	changed := false
	for i, callsign := range via {
		rec, exists := s.records[callsign]
		if !exists {
			continue
		}
		if i == len(via)-1 {
			applyFreshness(&rec, freshnessDirect, seenAt, rssi, snr)
		} else {
			applyFreshness(&rec, freshnessIndirect, seenAt, nil, nil)
		}
		s.setRecordLocked(callsign, rec)
		changed = true
	}
	return changed
}

func (s *Store) loadSQLite() error {
	rows, err := s.db.Query(`
		SELECT node_id, lat, lng, alt, hw_id, firstseen, lastseen, lastdirectseen, rssi, snr, via
		FROM nodes
	`)
	if err != nil {
		return fmt.Errorf("query positions: %w", err)
	}
	defer rows.Close()

	records := make(map[string]Record)
	for rows.Next() {
		nodeID, record, err := scanNodeRecord(rows)
		if err != nil {
			return err
		}
		records[nodeID] = record
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate positions: %w", err)
	}

	s.mu.Lock()
	s.records = records
	s.dirtyNodes = make(map[string]bool)
	s.dirty = false
	s.mu.Unlock()
	return nil
}

type nodeScanner interface {
	Scan(dest ...any) error
}

func scanNodeRecord(row nodeScanner) (string, Record, error) {
	var nodeID string
	var record Record
	var firstSeen, lastSeen string
	var lastDirectSeen sql.NullString
	var via sql.NullString
	if err := row.Scan(
		&nodeID,
		&record.Latitude,
		&record.Longitude,
		&record.Altitude,
		&record.HardwareID,
		&firstSeen,
		&lastSeen,
		&lastDirectSeen,
		&record.RSSI,
		&record.SNR,
		&via,
	); err != nil {
		return "", Record{}, fmt.Errorf("scan position: %w", err)
	}

	parsedFirstSeen, err := time.Parse(time.RFC3339Nano, firstSeen)
	if err != nil {
		return "", Record{}, fmt.Errorf("parse firstseen for %s: %w", nodeID, err)
	}
	parsedLastSeen, err := time.Parse(time.RFC3339Nano, lastSeen)
	if err != nil {
		return "", Record{}, fmt.Errorf("parse lastseen for %s: %w", nodeID, err)
	}
	record.FirstSeen = parsedFirstSeen
	record.LastSeen = parsedLastSeen

	if lastDirectSeen.Valid {
		parsedLastDirectSeen, err := time.Parse(time.RFC3339Nano, lastDirectSeen.String)
		if err != nil {
			return "", Record{}, fmt.Errorf("parse lastdirectseen for %s: %w", nodeID, err)
		}
		record.LastDirectSeen = &parsedLastDirectSeen
	}

	if via.Valid && via.String != "" {
		if err := json.Unmarshal([]byte(via.String), &record.Via); err != nil {
			return "", Record{}, fmt.Errorf("decode via for %s: %w", nodeID, err)
		}
	}
	if record.Via == nil {
		record.Via = []string{}
	}

	return nodeID, record, nil
}

func writeSQLite(db *sql.DB, records map[string]Record) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin positions sqlite save: %w", err)
	}
	for nodeID, record := range records {
		if err := upsertNode(tx, nodeID, record); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit positions sqlite save: %w", err)
	}
	return nil
}

func upsertNode(tx *sql.Tx, nodeID string, record Record) error {
	via, err := json.Marshal(normalizeVia(record.Via))
	if err != nil {
		return fmt.Errorf("encode via for %s: %w", nodeID, err)
	}

	var lastDirectSeen any
	if record.LastDirectSeen != nil {
		lastDirectSeen = record.LastDirectSeen.UTC().Format(time.RFC3339Nano)
	}

	_, err = tx.Exec(`
		INSERT INTO nodes(node_id, lat, lng, alt, hw_id, firstseen, lastseen, lastdirectseen, rssi, snr, via)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			lat = excluded.lat,
			lng = excluded.lng,
			alt = excluded.alt,
			hw_id = excluded.hw_id,
			firstseen = excluded.firstseen,
			lastseen = excluded.lastseen,
			lastdirectseen = excluded.lastdirectseen,
			rssi = excluded.rssi,
			snr = excluded.snr,
			via = excluded.via
	`,
		nodeID,
		record.Latitude,
		record.Longitude,
		record.Altitude,
		record.HardwareID,
		record.FirstSeen.UTC().Format(time.RFC3339Nano),
		record.LastSeen.UTC().Format(time.RFC3339Nano),
		lastDirectSeen,
		record.RSSI,
		record.SNR,
		string(via),
	)
	if err != nil {
		return fmt.Errorf("upsert node %s: %w", nodeID, err)
	}
	return nil
}

func normalizeVia(via []string) []string {
	if via == nil {
		return []string{}
	}
	return via
}
