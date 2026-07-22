package stats

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteSaveLoadRoundTrip(t *testing.T) {
	db := openStatsTestDB(t)
	now := time.Now().UTC().Truncate(time.Hour)
	store := NewSQLite(db, Config{RetentionDays: 30})
	store.RecordPacket(KindDM, now, nil)
	store.RecordPacket(KindPublic, now, nil)
	store.RecordDMAck(now)
	store.RecordChannel("broadcast", now)
	store.RecordChannel("ch:222", now)
	store.RecordChannel("dm:QQ1ABC-1", now)
	distance := 101.2
	store.RecordPacket(KindPosition, now, &distance)
	if err := store.save(); err != nil {
		t.Fatalf("save() error = %v", err)
	}

	loaded := NewSQLite(db, Config{RetentionDays: 30})
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	buckets, err := loaded.ReadRange(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ReadRange() error = %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("len(buckets) = %d, want 1", len(buckets))
	}
	got := buckets[0]
	if got.DM != 1 || got.Public != 1 || got.Position != 1 || got.DMAck != 1 || got.Total != 3 {
		t.Fatalf("bucket counts = %+v", got)
	}
	if !reflect.DeepEqual(got.Channels, map[string]int{"broadcast": 1, "ch:222": 1, "dm:QQ1ABC-1": 1}) {
		t.Fatalf("Channels = %#v", got.Channels)
	}
	if !reflect.DeepEqual(got.DistanceKm, map[string]int{"100+": 1}) {
		t.Fatalf("DistanceKm = %#v", got.DistanceKm)
	}
}

func TestSQLiteSaveIfCleanNoop(t *testing.T) {
	db := openStatsTestDB(t)
	store := NewSQLite(db, Config{RetentionDays: 30})
	if err := store.save(); err != nil {
		t.Fatalf("save() error = %v", err)
	}
}

func openStatsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE stats_hourly (hour_unix INTEGER PRIMARY KEY, dm INTEGER NOT NULL, dm_ack INTEGER NOT NULL, public INTEGER NOT NULL, telemetry INTEGER NOT NULL, position INTEGER NOT NULL, errors INTEGER NOT NULL, total INTEGER NOT NULL)`,
		`CREATE TABLE stats_channels (hour_unix INTEGER NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('broadcast', 'channel', 'dm')), target TEXT NOT NULL, count INTEGER NOT NULL, PRIMARY KEY (hour_unix, kind, target), FOREIGN KEY (hour_unix) REFERENCES stats_hourly(hour_unix) ON DELETE CASCADE)`,
		`CREATE TABLE stats_distance (hour_unix INTEGER NOT NULL, bucket_start_km INTEGER NOT NULL, bucket_end_km INTEGER NOT NULL, count INTEGER NOT NULL, PRIMARY KEY (hour_unix, bucket_start_km), CHECK (bucket_start_km >= 0), CHECK (bucket_end_km = bucket_start_km + 5), FOREIGN KEY (hour_unix) REFERENCES stats_hourly(hour_unix) ON DELETE CASCADE)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("create stats table: %v", err)
		}
	}
	return db
}
