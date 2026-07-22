package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportStatsImportsJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	hour := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC).Unix()
	path := filepath.Join(t.TempDir(), "stats.json")
	writeFile(t, path, fmt.Sprintf(`{"%d":{"hour":%d,"dm":1,"dm_ack":2,"public":3,"telemetry":4,"position":5,"errors":6,"total":13,"channels":{"broadcast":7,"ch:222":8,"dm:QQ1ABC-1":9},"distance_km":{"0-5":10,"100+":11}}}`, hour, hour))

	if err := db.ImportStats(ctx, path); err != nil {
		t.Fatalf("ImportStats() error = %v", err)
	}

	var dm, dmAck, public, telemetry, position, errors, total int
	if err := db.SQL().QueryRowContext(ctx, `SELECT dm, dm_ack, public, telemetry, position, errors, total FROM stats_hourly WHERE hour_unix = ?`, hour).Scan(&dm, &dmAck, &public, &telemetry, &position, &errors, &total); err != nil {
		t.Fatalf("query stats_hourly: %v", err)
	}
	if dm != 1 || dmAck != 2 || public != 3 || telemetry != 4 || position != 5 || errors != 6 || total != 13 {
		t.Fatalf("hourly counts = %d/%d/%d/%d/%d/%d/%d", dm, dmAck, public, telemetry, position, errors, total)
	}
	assertStatsCount(t, db, `SELECT count FROM stats_channels WHERE hour_unix = ? AND kind = 'channel' AND target = '222'`, hour, 8)
	assertStatsCount(t, db, `SELECT count FROM stats_distance WHERE hour_unix = ? AND bucket_start_km = 100 AND bucket_end_km = 105`, hour, 11)
	assertImportRecorded(t, db.SQL(), statsImportSource, 1)
}

func TestImportStatsMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportStats(ctx, filepath.Join(t.TempDir(), "stats.json")); err != nil {
		t.Fatalf("ImportStats() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), statsImportSource, 0)
}

func TestImportStatsInvalidJSONFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "stats.json")
	writeFile(t, path, "not-json")
	err := db.ImportStats(ctx, path)
	if err == nil {
		t.Fatal("ImportStats() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode stats import") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestImportStatsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	hour := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC).Unix()
	path := filepath.Join(t.TempDir(), "stats.json")
	writeFile(t, path, fmt.Sprintf(`{"%d":{"hour":%d,"dm":1,"total":1}}`, hour, hour))
	if err := db.ImportStats(ctx, path); err != nil {
		t.Fatalf("first ImportStats() error = %v", err)
	}
	writeFile(t, path, fmt.Sprintf(`{"%d":{"hour":%d,"dm":9,"total":9}}`, hour, hour))
	if err := db.ImportStats(ctx, path); err != nil {
		t.Fatalf("second ImportStats() error = %v", err)
	}
	assertStatsCount(t, db, `SELECT dm FROM stats_hourly WHERE hour_unix = ?`, hour, 1)
}

func assertStatsCount(t *testing.T, db *DB, query string, hour int64, want int) {
	t.Helper()
	var got int
	if err := db.SQL().QueryRowContext(context.Background(), query, hour).Scan(&got); err != nil {
		t.Fatalf("query stats count: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d", got, want)
	}
}
