package positions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
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
	mu      sync.Mutex
	path    string
	records map[string]Record
	dirty   bool
}

func New(path string) *Store {
	return &Store{
		path:    path,
		records: make(map[string]Record),
	}
}

func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "nodes", "positions.json")
}

func (s *Store) Load() error {
	// Remove any leftover temp file from a previous crash.
	_ = os.Remove(s.path + ".tmp")

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open positions: %w", err)
	}
	defer file.Close()

	var records map[string]Record
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		return fmt.Errorf("decode positions: %w", err)
	}

	s.mu.Lock()
	s.records = records
	if s.records == nil {
		s.records = make(map[string]Record)
	}
	s.dirty = false
	s.mu.Unlock()

	return nil
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
		applyFreshness(&record, freshnessIndirect, seenAt, 0, 0)
		if len(via) > 0 {
			s.touchLastHopLocked(via[len(via)-1], position.RSSI, position.SNR, seenAt)
		}
	}

	if exists && reflect.DeepEqual(current, record) {
		return false
	}

	s.records[callsign] = record
	s.dirty = true
	return true
}

// TouchFromPacket updates freshness for msg/tele packets without changing
// coordinates. Only updates records that already exist — never creates new ones.
// Direct packets: full freshness (lastSeen, lastDirectSeen, rssi, snr) on origin.
// Indirect packets: lastSeen only on origin; full freshness on last relay.
func (s *Store) TouchFromPacket(src string, rssi, snr int, seenAt time.Time) bool {
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
			s.records[callsign] = rec
			s.dirty = true
			changed = true
		}
	} else {
		if rec, exists := s.records[callsign]; exists {
			applyFreshness(&rec, freshnessIndirect, seenAt, 0, 0)
			s.records[callsign] = rec
			s.dirty = true
			changed = true
		}
		if len(via) > 0 {
			changed = s.touchLastHopLocked(via[len(via)-1], rssi, snr, seenAt) || changed
		}
	}
	return changed
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
	records := make(map[string]Record, len(s.records))
	for callsign, record := range s.records {
		records[callsign] = record
	}
	s.dirty = false
	s.mu.Unlock()

	if err := writeFileAtomically(s.path, records); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
	}

	return nil
}

type freshnessMode int

const (
	freshnessIndirect freshnessMode = iota
	freshnessDirect
)

// applyFreshness updates lastSeen and, for direct mode, lastDirectSeen/rssi/snr.
// For indirect mode only lastSeen is updated; rssi/snr/lastDirectSeen are left as-is.
func applyFreshness(rec *Record, mode freshnessMode, seenAt time.Time, rssi, snr int) {
	rec.LastSeen = seenAt.UTC()
	if mode == freshnessDirect {
		t := seenAt.UTC()
		rec.LastDirectSeen = &t
		rec.RSSI = rssi
		rec.SNR = snr
	}
}

// touchLastHopLocked applies direct freshness to an existing record.
// Caller must hold s.mu.
func (s *Store) touchLastHopLocked(callsign string, rssi, snr int, seenAt time.Time) bool {
	rec, exists := s.records[callsign]
	if !exists {
		return false
	}
	applyFreshness(&rec, freshnessDirect, seenAt, rssi, snr)
	s.records[callsign] = rec
	s.dirty = true
	return true
}

func writeFileAtomically(path string, records map[string]Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create positions dir: %w", err)
	}

	tmpPath := path + ".tmp"
	temp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp positions file: %w", err)
	}

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if encErr := encoder.Encode(records); encErr != nil {
		temp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode positions: %w", encErr)
	}

	if err := temp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp positions file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace positions file: %w", err)
	}

	return nil
}
