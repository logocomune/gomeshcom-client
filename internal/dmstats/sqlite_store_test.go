package dmstats

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteLoadRoundTrip(t *testing.T) {
	db := openDMStatsTestDB(t)
	s1 := NewSQLite(db)
	s1.RecordSent("QQ0XX-1")
	s1.RecordAck("QQ0XX-1")
	if err := s1.save(); err != nil {
		t.Fatalf("save() error = %v", err)
	}

	s2 := NewSQLite(db)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	snap := s2.Snapshot()
	if snap["QQ0XX-1"].Sent != 1 || snap["QQ0XX-1"].Ack != 1 {
		t.Fatalf("full entry = %+v", snap["QQ0XX-1"])
	}
	if snap["QQ0XX"].Sent != 1 || snap["QQ0XX"].Ack != 1 {
		t.Fatalf("base entry = %+v", snap["QQ0XX"])
	}
}

func TestSQLiteSaveReplacesRemovedEntries(t *testing.T) {
	db := openDMStatsTestDB(t)
	s := NewSQLite(db)
	s.RecordSent("CALL-1")
	if err := s.save(); err != nil {
		t.Fatalf("first save() error = %v", err)
	}
	s.mu.Lock()
	delete(s.entries, "CALL-1")
	s.dirty = true
	s.mu.Unlock()
	if err := s.save(); err != nil {
		t.Fatalf("second save() error = %v", err)
	}

	loaded := NewSQLite(db)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := loaded.Snapshot()["CALL-1"]; ok {
		t.Fatal("CALL-1 should have been removed")
	}
}

func openDMStatsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE dm_stats (
			callsign TEXT PRIMARY KEY,
			sent INTEGER NOT NULL,
			ack INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create dm_stats table: %v", err)
	}
	return db
}
