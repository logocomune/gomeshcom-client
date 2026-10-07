package chatstatus

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"

	_ "modernc.org/sqlite"
)

func TestPropertyStoreNoticePreservesExistingStatus(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		entry := Entry{
			LastMsgReceived: time.Unix(rapid.Int64Range(0, 2000000000).Draw(t, "received"), 0).UTC(),
			LastRead:        time.Unix(rapid.Int64Range(0, 2000000000).Draw(t, "read"), 0).UTC(),
			UnreadCount:     rapid.IntRange(0, 1000).Draw(t, "unread"),
			LastMsg:         rapid.String().Draw(t, "message"),
		}
		snapshot := map[string]Entry{"P_broadcast": entry}
		applySnapshotRecord(snapshot, map[string]time.Time{}, "P_broadcast", time.Now(), "QQ0QQ-1  :sto156 QQ1ABC-1")
		if snapshot["P_broadcast"] != entry {
			t.Fatalf("custody notice changed status: %+v", snapshot)
		}
	})
}

var (
	t0 = time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Minute)
)

func TestSQLiteSnapshotExcludesHistoricalStoreNotices(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprint(direct), func(t *testing.T) {
			db := openChatStatusTestDB(t)
			conversation := "P_broadcast"
			insert := func(at time.Time, message string) {
				t.Helper()
				if !direct {
					insertPublicChatRow(t, db, conversation, at, message)
					return
				}
				_, err := db.Exec(`INSERT INTO chats_dm(conversation_id, received_at, src, dst, msg) VALUES ('DM_QQ0QQ_QQ1ABC-1', ?, 'QQ1ABC-1', 'QQ0QQ-2', ?)`, at.Format(time.RFC3339Nano), message)
				if err != nil {
					t.Fatal(err)
				}
			}
			if direct {
				conversation = "DM_QQ0QQ-2_QQ1ABC-1"
			}
			insert(t1, "QQ0QQ-2  :sto156 QQ2ABC-1")
			store, err := NewSQLite(db)
			if err != nil {
				t.Fatal(err)
			}
			if entry, ok := store.Snapshot()[conversation]; ok {
				t.Fatalf("notice-only status = %+v", entry)
			}
			insert(t0, "hello")
			entry := store.Snapshot()[conversation]
			if entry.UnreadCount != 1 || entry.LastMsg != "hello" || !entry.LastMsgReceived.Equal(t0) {
				t.Fatalf("status = %+v", entry)
			}
			store.MarkRead(conversation, t0)
			if entry := store.Snapshot()[conversation]; entry.UnreadCount != 0 || !entry.LastRead.Equal(t0) {
				t.Fatalf("read status = %+v", entry)
			}
		})
	}
}

func TestSQLiteSaveLoadRoundTrip(t *testing.T) {
	db := openChatStatusTestDB(t)

	s1, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	s1.MarkRead("P_broadcast", t1)
	s1.RecordIncoming("P_1", t0, "hello")
	if err := s1.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	s2, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	snap := s2.Snapshot()
	if !snap["P_broadcast"].LastRead.Equal(t1) {
		t.Fatalf("P_broadcast LastRead = %v, want %v", snap["P_broadcast"].LastRead, t1)
	}
	if _, ok := snap["P_1"]; ok {
		t.Fatal("P_1 should not persist without LastRead; unread state is derived from chat rows")
	}
}

func TestSQLiteSnapshotDerivesUnreadFromChatRows(t *testing.T) {
	db := openChatStatusTestDB(t)
	insertPublicChatRow(t, db, "P_broadcast", t0, "old")
	insertPublicChatRow(t, db, "P_broadcast", t1, "new")

	store, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	store.MarkRead("P_broadcast", t0)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	snap := store.Snapshot()
	entry := snap["P_broadcast"]
	if entry.UnreadCount != 1 {
		t.Fatalf("UnreadCount = %d, want 1", entry.UnreadCount)
	}
	if !entry.LastMsgReceived.Equal(t1) {
		t.Fatalf("LastMsgReceived = %v, want %v", entry.LastMsgReceived, t1)
	}
	if entry.LastMsg != "new" {
		t.Fatalf("LastMsg = %q, want new", entry.LastMsg)
	}
}

func TestSQLiteSnapshotDerivesDMStatusKey(t *testing.T) {
	db := openChatStatusTestDB(t)
	_, err := db.Exec(`
		INSERT INTO chats_dm(conversation_id, received_at, src, dst, msg)
		VALUES ('DM_QQ0QQ_QQ1ABC-1', ?, 'QQ1ABC-1', 'QQ0QQ-2', 'dm')
	`, t1.UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert dm row: %v", err)
	}

	store, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	snap := store.Snapshot()
	entry, ok := snap["DM_QQ0QQ-2_QQ1ABC-1"]
	if !ok {
		t.Fatalf("DM status key missing: %v", snap)
	}
	if entry.UnreadCount != 1 || entry.LastMsg != "dm" {
		t.Fatalf("entry = %+v, want unread dm", entry)
	}
}

func TestSQLiteRemoveDeletesReadState(t *testing.T) {
	db := openChatStatusTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	s.MarkRead("P_broadcast", t1)
	if err := s.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}
	s.Remove("P_broadcast")
	if err := s.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() after remove error = %v", err)
	}

	loaded, err := NewSQLite(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Snapshot()["P_broadcast"]; ok {
		t.Fatal("P_broadcast present after remove")
	}
}

func openChatStatusTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE chat_reads (
			conversation_id TEXT PRIMARY KEY,
			last_read TEXT NOT NULL
		)`,
		`CREATE TABLE chats_public (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			received_at TEXT NOT NULL,
			msg TEXT NOT NULL
		)`,
		`CREATE TABLE chats_dm (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			received_at TEXT NOT NULL,
			src TEXT,
			dst TEXT NOT NULL,
			msg TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("create chatstatus table: %v", err)
		}
	}
	return db
}

func insertPublicChatRow(t *testing.T, db *sql.DB, convID string, at time.Time, msg string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO chats_public(conversation_id, received_at, msg) VALUES (?, ?, ?)`, convID, at.UTC().Format(time.RFC3339Nano), msg)
	if err != nil {
		t.Fatalf("insert public row: %v", err)
	}
}
