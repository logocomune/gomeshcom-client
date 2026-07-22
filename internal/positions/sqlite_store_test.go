package positions

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	_ "modernc.org/sqlite"
)

func TestSQLiteStoreSaveAndLoad(t *testing.T) {
	ctx := context.Background()
	db := openPositionsTestDB(t, ctx)
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	store := NewSQLite(db)
	store.Update(meshcom.Position{
		Source:     "QQ1ABC-1",
		Latitude:   48.1,
		Longitude:  16.3,
		Altitude:   123,
		HardwareID: "TLORA_V2",
		RSSI:       intPtr(-90),
		SNR:        intPtr(8),
	}, seenAt)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	loaded := NewSQLite(db)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got, ok := loaded.Get("QQ1ABC-1")
	if !ok {
		t.Fatal("Get(QQ1ABC-1) ok = false")
	}
	want := store.Snapshot()["QQ1ABC-1"]
	if !recordsEqual(got, want) {
		t.Fatalf("loaded = %+v, want %+v", got, want)
	}
}

func TestSQLiteStoreTouchPersists(t *testing.T) {
	ctx := context.Background()
	db := openPositionsTestDB(t, ctx)
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)

	store := NewSQLite(db)
	store.Update(meshcom.Position{Source: "A-1", Latitude: 43, Longitude: 10}, t0)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("initial SaveIfDirty() error = %v", err)
	}
	store.TouchFromPacket("A-1", intPtr(-80), intPtr(7), t1)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("touch SaveIfDirty() error = %v", err)
	}

	loaded := NewSQLite(db)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	record := loaded.Snapshot()["A-1"]
	if record.LastSeen != t1 || record.LastDirectSeen == nil || !record.LastDirectSeen.Equal(t1) {
		t.Fatalf("freshness = %+v, want lastSeen/lastDirectSeen %v", record, t1)
	}
	if record.RSSI != -80 || record.SNR != 7 {
		t.Fatalf("RSSI/SNR = %d/%d, want -80/7", record.RSSI, record.SNR)
	}
}

func TestSQLiteStoreSavesOnlyChangedNodes(t *testing.T) {
	ctx := context.Background()
	db := openPositionsTestDB(t, ctx)
	installUpdateAudit(t, db, ctx)
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	store := NewSQLite(db)
	store.Update(meshcom.Position{Source: "A-1", Latitude: 43, Longitude: 10}, seenAt)
	store.Update(meshcom.Position{Source: "B-1", Latitude: 44, Longitude: 11}, seenAt)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("initial SaveIfDirty() error = %v", err)
	}
	clearUpdateAudit(t, db, ctx)

	store.TouchFromPacket("A-1", intPtr(-80), intPtr(7), seenAt.Add(time.Minute))
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("second SaveIfDirty() error = %v", err)
	}

	if got := updateAuditCount(t, db, ctx); got != 1 {
		t.Fatalf("updated rows = %d, want 1", got)
	}
	assertUpdatedNode(t, db, ctx, "A-1")
}

func TestSQLiteStoreKeepsChangedNodesDirtyAfterSaveFailure(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gomeshcom.db")
	db := openPositionsTestDBAtPath(t, ctx, path)
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	store := NewSQLite(db)
	store.Update(meshcom.Position{Source: "A-1", Latitude: 43, Longitude: 10}, seenAt)
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("initial SaveIfDirty() error = %v", err)
	}

	store.TouchFromPacket("A-1", intPtr(-80), intPtr(7), seenAt.Add(time.Minute))
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := store.SaveIfDirty(); err == nil {
		t.Fatal("SaveIfDirty() error = nil, want failure")
	}

	reopened, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer reopened.Close()
	store.db = reopened
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("retry SaveIfDirty() error = %v", err)
	}

	loaded := NewSQLite(reopened)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	record := loaded.Snapshot()["A-1"]
	if record.RSSI != -80 || record.SNR != 7 {
		t.Fatalf("RSSI/SNR = %d/%d, want -80/7", record.RSSI, record.SNR)
	}
}

func TestSQLiteStoreSaveIfDirtyNoop(t *testing.T) {
	ctx := context.Background()
	db := openPositionsTestDB(t, ctx)
	store := NewSQLite(db)

	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}
}

func openPositionsTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	return openPositionsTestDBAtPath(t, ctx, filepath.Join(t.TempDir(), "gomeshcom.db"))
}

func openPositionsTestDBAtPath(t *testing.T, ctx context.Context, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE nodes (
			node_id TEXT PRIMARY KEY,
			lat REAL,
			lng REAL,
			alt INTEGER,
			hw_id TEXT,
			firstseen TEXT,
			lastseen TEXT,
			lastdirectseen TEXT,
			rssi INTEGER,
			snr INTEGER,
			via TEXT CHECK (via IS NULL OR (json_valid(via) AND json_type(via) = 'array'))
		)
	`); err != nil {
		t.Fatalf("create nodes table: %v", err)
	}
	return db
}

func installUpdateAudit(t *testing.T, db *sql.DB, ctx context.Context) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `CREATE TABLE node_update_audit (node_id TEXT NOT NULL)`); err != nil {
		t.Fatalf("create node_update_audit: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER node_update_audit_trigger
		AFTER UPDATE ON nodes
		BEGIN
			INSERT INTO node_update_audit(node_id) VALUES (NEW.node_id);
		END
	`); err != nil {
		t.Fatalf("create node_update_audit_trigger: %v", err)
	}
}

func clearUpdateAudit(t *testing.T, db *sql.DB, ctx context.Context) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `DELETE FROM node_update_audit`); err != nil {
		t.Fatalf("clear node_update_audit: %v", err)
	}
}

func updateAuditCount(t *testing.T, db *sql.DB, ctx context.Context) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_update_audit`).Scan(&count); err != nil {
		t.Fatalf("count node_update_audit: %v", err)
	}
	return count
}

func assertUpdatedNode(t *testing.T, db *sql.DB, ctx context.Context, want string) {
	t.Helper()
	var nodeID string
	if err := db.QueryRowContext(ctx, `SELECT node_id FROM node_update_audit`).Scan(&nodeID); err != nil {
		t.Fatalf("query node_update_audit: %v", err)
	}
	if nodeID != want {
		t.Fatalf("updated node = %s, want %s", nodeID, want)
	}
}
