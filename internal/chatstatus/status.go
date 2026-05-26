package chatstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const saveInterval = time.Minute

// Entry tracks the read/unread state for a single conversation thread.
type Entry struct {
	LastMsgReceived time.Time `json:"lastMsgReceived"`
	LastRead        time.Time `json:"lastRead"`
	UnreadCount     int       `json:"unreadCount"`
	LastMsg         string    `json:"lastMsg,omitempty"`
}

// Store holds per-conversation chat status in memory and persists it periodically.
type Store struct {
	mu      sync.Mutex
	path    string
	entries map[string]*Entry
	dirty   bool
	clock   func() time.Time
}

// New creates a Store backed by path. Existing data is loaded if the file exists.
func New(path string) (*Store, error) {
	s := &Store{
		path:    path,
		entries: make(map[string]*Entry),
		clock:   time.Now,
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Load reads the status file from disk. A missing file is silently ignored.
func (s *Store) Load() error {
	_ = os.Remove(s.path + ".tmp")

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open chat status: %w", err)
	}
	defer file.Close()

	var entries map[string]*Entry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return fmt.Errorf("decode chat status: %w", err)
	}

	s.mu.Lock()
	s.entries = entries
	if s.entries == nil {
		s.entries = make(map[string]*Entry)
	}
	s.dirty = false
	s.mu.Unlock()

	return nil
}

// RecordIncoming increments the unread counter, updates LastMsgReceived, and stores
// the message text preview for convID.
func (s *Store) RecordIncoming(convID string, ts time.Time, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.getOrCreate(convID)
	e.UnreadCount++
	e.LastMsgReceived = ts.UTC()
	e.LastMsg = msg
	s.dirty = true
}

// MarkRead zeroes the unread counter and sets LastRead for convID.
func (s *Store) MarkRead(convID string, ts time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.getOrCreate(convID)
	e.UnreadCount = 0
	e.LastRead = ts.UTC()
	s.dirty = true
}

// Remove deletes the status entry for convID from the store.
// It is a no-op when the entry does not exist.
func (s *Store) Remove(convID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[convID]; ok {
		delete(s.entries, convID)
		s.dirty = true
	}
}

// Snapshot returns a deep copy of the current status map.
func (s *Store) Snapshot() map[string]Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]Entry, len(s.entries))
	for k, v := range s.entries {
		out[k] = *v
	}
	return out
}

// Start runs the periodic save loop until ctx is cancelled, then flushes.
func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("chat status flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("chat status save failed", "error", err)
			}
		}
	}
}

// SaveIfDirty persists the store atomically when there are pending changes.
func (s *Store) SaveIfDirty() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	snapshot := make(map[string]*Entry, len(s.entries))
	for k, v := range s.entries {
		cp := *v
		snapshot[k] = &cp
	}
	s.dirty = false
	s.mu.Unlock()

	if err := writeFileAtomically(s.path, snapshot); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
	}

	return nil
}

func (s *Store) getOrCreate(convID string) *Entry {
	e, ok := s.entries[convID]
	if !ok {
		e = &Entry{}
		s.entries[convID] = e
	}
	return e
}

func writeFileAtomically(path string, entries map[string]*Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chat status dir: %w", err)
	}

	tmpPath := path + ".tmp"
	temp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp chat status file: %w", err)
	}

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if encErr := encoder.Encode(entries); encErr != nil {
		temp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode chat status: %w", encErr)
	}

	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp chat status file: %w", err)
	}

	if err := temp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp chat status file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace chat status file: %w", err)
	}

	return nil
}
