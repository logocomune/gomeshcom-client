package receivelog

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteLoggerAppendReadSince(t *testing.T) {
	db := openReceiveLogTestDB(t)
	logger := NewSQLite(Config{Enabled: true, Path: "sqlite"}, db)
	cutoff := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	if err := logger.Append(Record{ReceivedAt: cutoff.Add(-time.Second), RemoteAddr: "old", Bytes: 1, Raw: `{"type":"msg","msg":"old"}`, PacketType: "msg"}); err != nil {
		t.Fatalf("append old: %v", err)
	}
	if err := logger.Append(Record{ReceivedAt: cutoff, RemoteAddr: "new", Bytes: 2, Raw: `{"type":"msg","msg":"new"}`, PacketType: "msg"}); err != nil {
		t.Fatalf("append new: %v", err)
	}

	records, err := logger.ReadSince(cutoff)
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if records[0].Raw != `{"type":"msg","msg":"new"}` || records[0].PacketType != "msg" || records[0].RemoteAddr != "new" || records[0].Bytes != 2 {
		t.Fatalf("record = %+v", records[0])
	}
}

func TestSQLiteLoggerDisabled(t *testing.T) {
	db := openReceiveLogTestDB(t)
	logger := NewSQLite(Config{Enabled: false, Path: "sqlite"}, db)
	if err := logger.Append(Record{Raw: `{"type":"msg"}`}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	records, err := logger.ReadSince(time.Time{})
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("record count = %d, want 0", len(records))
	}
}

func TestSQLiteLoggerAppendDoesNotPruneRetention(t *testing.T) {
	db := openReceiveLogTestDB(t)
	logger := NewSQLite(Config{Enabled: true, Path: "sqlite", RetentionDays: 365}, db)
	now := time.Now().UTC()

	if err := logger.Append(Record{ReceivedAt: now.AddDate(0, 0, -365), Raw: `{"type":"msg","msg":"expired"}`, PacketType: "msg"}); err != nil {
		t.Fatalf("append expired: %v", err)
	}
	if err := logger.Append(Record{ReceivedAt: now, Raw: `{"type":"msg","msg":"fresh"}`, PacketType: "msg"}); err != nil {
		t.Fatalf("append fresh: %v", err)
	}

	records, err := logger.ReadSince(time.Time{})
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
}

func openReceiveLogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE receive_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			received_at TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			bytes INTEGER NOT NULL,
			raw TEXT NOT NULL,
			packet_type TEXT,
			parse_error TEXT
		)
	`); err != nil {
		t.Fatalf("create receive_log table: %v", err)
	}
	return db
}
