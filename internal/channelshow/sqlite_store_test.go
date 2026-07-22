package channelshow

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteSaveLoadRoundTrip(t *testing.T) {
	db := openChannelShowTestDB(t)
	store, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	want, err := store.Update(Config{Mode: ModeAllowlist, Channels: []string{"*", "222", "222"}})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	loaded, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite loaded error = %v", err)
	}
	if got := loaded.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestSQLiteSaveIfDirtyNoop(t *testing.T) {
	db := openChannelShowTestDB(t)
	store, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}
}

func openChannelShowTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE channel_show (id INTEGER PRIMARY KEY CHECK (id = 1), mode TEXT NOT NULL)`,
		`CREATE TABLE channel_show_channels (channel TEXT PRIMARY KEY, last_message_at TEXT)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("create channelshow table: %v", err)
		}
	}
	return db
}
