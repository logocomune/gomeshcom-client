package chatstatus

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
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
	db      *sql.DB
	entries map[string]*Entry
	dirty   bool
	clock   func() time.Time
}

func NewSQLite(db *sql.DB) (*Store, error) {
	s := &Store{
		db:      db,
		entries: make(map[string]*Entry),
		clock:   time.Now,
	}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Load reads chat read markers from SQLite.
func (s *Store) Load() error {
	return s.loadSQLite()
}

// RecordIncoming increments the unread counter, updates LastMsgReceived, and stores
// the message text preview for convID.
func (s *Store) RecordIncoming(convID string, ts time.Time, msg string) {
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
	snapshot, err := s.snapshotSQLite()
	if err != nil {
		return map[string]Entry{}
	}
	return snapshot
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

	if err := writeSQLite(s.db, snapshot); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
	}

	return nil
}

func (s *Store) loadSQLite() error {
	rows, err := s.db.Query(`SELECT conversation_id, last_read FROM chat_reads`)
	if err != nil {
		return fmt.Errorf("query chat reads: %w", err)
	}
	defer rows.Close()

	entries := make(map[string]*Entry)
	for rows.Next() {
		var convID string
		var lastRead string
		if err := rows.Scan(&convID, &lastRead); err != nil {
			return fmt.Errorf("scan chat read: %w", err)
		}
		parsedLastRead, err := time.Parse(time.RFC3339Nano, lastRead)
		if err != nil {
			return fmt.Errorf("parse chat read %s: %w", convID, err)
		}
		entries[convID] = &Entry{LastRead: parsedLastRead}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate chat reads: %w", err)
	}

	s.mu.Lock()
	s.entries = entries
	s.dirty = false
	s.mu.Unlock()
	return nil
}

func (s *Store) snapshotSQLite() (map[string]Entry, error) {
	reads := s.SnapshotReads()
	snapshot := make(map[string]Entry)
	if err := applyPublicSnapshot(s.db, snapshot, reads); err != nil {
		return nil, err
	}
	if err := applyDMSnapshot(s.db, snapshot, reads); err != nil {
		return nil, err
	}
	for convID, read := range reads {
		entry := snapshot[convID]
		entry.LastRead = read
		snapshot[convID] = entry
	}
	return snapshot, nil
}

func (s *Store) SnapshotReads() map[string]time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	reads := make(map[string]time.Time, len(s.entries))
	for convID, entry := range s.entries {
		reads[convID] = entry.LastRead
	}
	return reads
}

func applyPublicSnapshot(db *sql.DB, snapshot map[string]Entry, reads map[string]time.Time) error {
	rows, err := db.Query(`
		SELECT conversation_id, received_at, msg
		FROM chats_public
		ORDER BY received_at, id
	`)
	if err != nil {
		return fmt.Errorf("query public chat status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var convID string
		var receivedAt string
		var msg string
		if err := rows.Scan(&convID, &receivedAt, &msg); err != nil {
			return fmt.Errorf("scan public chat status: %w", err)
		}
		at, err := time.Parse(time.RFC3339Nano, receivedAt)
		if err != nil {
			return fmt.Errorf("parse public chat status time: %w", err)
		}
		applySnapshotRecord(snapshot, reads, convID, at, msg)
	}
	return rows.Err()
}

func applyDMSnapshot(db *sql.DB, snapshot map[string]Entry, reads map[string]time.Time) error {
	rows, err := db.Query(`
		SELECT conversation_id, received_at, src, dst, msg
		FROM chats_dm
		ORDER BY received_at, id
	`)
	if err != nil {
		return fmt.Errorf("query dm chat status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var convID string
		var receivedAt string
		var src sql.NullString
		var dst string
		var msg string
		if err := rows.Scan(&convID, &receivedAt, &src, &dst, &msg); err != nil {
			return fmt.Errorf("scan dm chat status: %w", err)
		}
		at, err := time.Parse(time.RFC3339Nano, receivedAt)
		if err != nil {
			return fmt.Errorf("parse dm chat status time: %w", err)
		}
		statusKey := dmStatusKey(convID, src.String, dst)
		if statusKey == "" {
			continue
		}
		applySnapshotRecord(snapshot, reads, statusKey, at, msg)
	}
	return rows.Err()
}

func applySnapshotRecord(snapshot map[string]Entry, reads map[string]time.Time, convID string, receivedAt time.Time, msg string) {
	entry := snapshot[convID]
	entry.LastRead = reads[convID]
	entry.LastMsgReceived = receivedAt.UTC()
	entry.LastMsg = msg
	if entry.LastRead.IsZero() || receivedAt.After(entry.LastRead) {
		entry.UnreadCount++
	}
	snapshot[convID] = entry
}

func dmStatusKey(conversationID string, src string, dst string) string {
	if !strings.HasPrefix(conversationID, "DM_") {
		return ""
	}
	rest := strings.TrimPrefix(conversationID, "DM_")
	idx := strings.Index(rest, "_")
	if idx < 0 {
		return conversationID
	}
	localBase := rest[:idx]
	peer := rest[idx+1:]
	srcOrigin := strings.ToUpper(strings.SplitN(src, ",", 2)[0])
	dstUpper := strings.ToUpper(dst)
	if baseCall(srcOrigin) == localBase {
		return "DM_" + sanitize(srcOrigin) + "_" + sanitize(peer)
	}
	if baseCall(dstUpper) == localBase {
		return "DM_" + sanitize(dstUpper) + "_" + sanitize(peer)
	}
	return conversationID
}

func baseCall(callsign string) string {
	if i := strings.LastIndex(callsign, "-"); i >= 0 && isNumeric(callsign[i+1:]) {
		return callsign[:i]
	}
	return callsign
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var unsafeStatusChars = strings.NewReplacer("/", "_", "\\", "_", " ", "_")

func sanitize(value string) string {
	return unsafeStatusChars.Replace(strings.ToUpper(value))
}

func writeSQLite(db *sql.DB, entries map[string]*Entry) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin chat reads save: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM chat_reads`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear chat reads: %w", err)
	}
	for convID, entry := range entries {
		if entry.LastRead.IsZero() {
			continue
		}
		_, err := tx.Exec(`INSERT INTO chat_reads(conversation_id, last_read) VALUES (?, ?)`, convID, entry.LastRead.UTC().Format(time.RFC3339Nano))
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save chat read %s: %w", convID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat reads save: %w", err)
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
