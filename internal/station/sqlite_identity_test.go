package station_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/station"
	_ "modernc.org/sqlite"
)

func TestNewSQLiteUsesConfigDefault(t *testing.T) {
	db := openStationTestDB(t)
	id, err := station.NewSQLite(db, "IU5PMP-1")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	if got := id.Current(); got != "IU5PMP-1" {
		t.Fatalf("Current() = %q, want IU5PMP-1", got)
	}
}

func TestNewSQLiteLoadsPersisted(t *testing.T) {
	db := openStationTestDB(t)
	if _, err := db.ExecContext(context.Background(), `INSERT INTO station_identity(id, callsign) VALUES (1, 'QQ1ABC-1')`); err != nil {
		t.Fatalf("insert station identity: %v", err)
	}

	id, err := station.NewSQLite(db, "IU5PMP-1")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	if got := id.Current(); got != "QQ1ABC-1" {
		t.Fatalf("Current() = %q, want QQ1ABC-1", got)
	}
}

func TestSQLiteSaveIfDirtyRoundTrip(t *testing.T) {
	db := openStationTestDB(t)
	id, err := station.NewSQLite(db, "IU5PMP-1")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	if _, err := id.Update("qq0qq-2"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := id.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	loaded, err := station.NewSQLite(db, "IU5PMP-1")
	if err != nil {
		t.Fatalf("NewSQLite() reload error = %v", err)
	}
	if got := loaded.Current(); got != "QQ0QQ-2" {
		t.Fatalf("Current() = %q, want QQ0QQ-2", got)
	}
}

func TestSQLiteSaveIfDirtyNoopWhenClean(t *testing.T) {
	db := openStationTestDB(t)
	id, err := station.NewSQLite(db, "IU5PMP-1")
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	if err := id.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM station_identity`).Scan(&count); err != nil {
		t.Fatalf("query station_identity: %v", err)
	}
	if count != 0 {
		t.Fatalf("station_identity rows = %d, want 0", count)
	}
}

func openStationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE station_identity (id INTEGER PRIMARY KEY CHECK (id = 1), callsign TEXT NOT NULL)`); err != nil {
		t.Fatalf("create station_identity: %v", err)
	}
	return db
}
