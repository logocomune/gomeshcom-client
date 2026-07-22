package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportDMStatsImportsJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "dm_stats.json")
	writeFile(t, path, `{"CALL-1":{"sent":3,"ack":2},"CALL":{"sent":3,"ack":2}}`)

	if err := db.ImportDMStats(ctx, path); err != nil {
		t.Fatalf("ImportDMStats() error = %v", err)
	}

	var sent, ack int
	if err := db.SQL().QueryRowContext(ctx, `SELECT sent, ack FROM dm_stats WHERE callsign = 'CALL-1'`).Scan(&sent, &ack); err != nil {
		t.Fatalf("query dm_stats: %v", err)
	}
	if sent != 3 || ack != 2 {
		t.Fatalf("sent/ack = %d/%d, want 3/2", sent, ack)
	}
	assertImportRecorded(t, db.SQL(), dmStatsImportSource, 2)
}

func TestImportDMStatsMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportDMStats(ctx, filepath.Join(t.TempDir(), "dm_stats.json")); err != nil {
		t.Fatalf("ImportDMStats() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), dmStatsImportSource, 0)
}

func TestImportDMStatsInvalidJSONFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "dm_stats.json")
	writeFile(t, path, "not-json")
	err := db.ImportDMStats(ctx, path)
	if err == nil {
		t.Fatal("ImportDMStats() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode dm stats import") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestImportDMStatsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "dm_stats.json")
	writeFile(t, path, `{"CALL":{"sent":1,"ack":0}}`)
	if err := db.ImportDMStats(ctx, path); err != nil {
		t.Fatalf("first ImportDMStats() error = %v", err)
	}
	writeFile(t, path, `{"CALL":{"sent":9,"ack":9}}`)
	if err := db.ImportDMStats(ctx, path); err != nil {
		t.Fatalf("second ImportDMStats() error = %v", err)
	}
	var sent, ack int
	if err := db.SQL().QueryRowContext(ctx, `SELECT sent, ack FROM dm_stats WHERE callsign = 'CALL'`).Scan(&sent, &ack); err != nil {
		t.Fatalf("query dm_stats: %v", err)
	}
	if sent != 1 || ack != 0 {
		t.Fatalf("sent/ack = %d/%d, want first import 1/0", sent, ack)
	}
}
