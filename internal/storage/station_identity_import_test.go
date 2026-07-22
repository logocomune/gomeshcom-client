package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportStationIdentityImportsJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "station.json")
	writeFile(t, path, `{"callsign":" qq1abc-1 "}`)

	if err := db.ImportStationIdentity(ctx, path); err != nil {
		t.Fatalf("ImportStationIdentity() error = %v", err)
	}

	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT callsign FROM station_identity WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("query station_identity: %v", err)
	}
	if got != "QQ1ABC-1" {
		t.Fatalf("callsign = %q, want QQ1ABC-1", got)
	}
	assertImportRecorded(t, db.SQL(), stationIdentityImportSource, 1)
}

func TestImportStationIdentityMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportStationIdentity(ctx, filepath.Join(t.TempDir(), "station.json")); err != nil {
		t.Fatalf("ImportStationIdentity() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), stationIdentityImportSource, 0)
}

func TestImportStationIdentityInvalidCallsignRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "station.json")
	writeFile(t, path, `{"callsign":"!!"}`)

	if err := db.ImportStationIdentity(ctx, path); err != nil {
		t.Fatalf("ImportStationIdentity() error = %v", err)
	}

	var count int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM station_identity`).Scan(&count); err != nil {
		t.Fatalf("query station_identity: %v", err)
	}
	if count != 0 {
		t.Fatalf("station_identity rows = %d, want 0", count)
	}
	assertImportRecorded(t, db.SQL(), stationIdentityImportSource, 0)
}

func TestImportStationIdentityInvalidJSONFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "station.json")
	writeFile(t, path, "not-json")

	err := db.ImportStationIdentity(ctx, path)
	if err == nil {
		t.Fatal("ImportStationIdentity() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode station identity import") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestImportStationIdentityIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "station.json")
	writeFile(t, path, `{"callsign":"QQ1ABC-1"}`)
	if err := db.ImportStationIdentity(ctx, path); err != nil {
		t.Fatalf("first ImportStationIdentity() error = %v", err)
	}
	writeFile(t, path, `{"callsign":"QQ2ABC-2"}`)
	if err := db.ImportStationIdentity(ctx, path); err != nil {
		t.Fatalf("second ImportStationIdentity() error = %v", err)
	}

	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT callsign FROM station_identity WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("query station_identity: %v", err)
	}
	if got != "QQ1ABC-1" {
		t.Fatalf("callsign = %q, want first import value", got)
	}
}
