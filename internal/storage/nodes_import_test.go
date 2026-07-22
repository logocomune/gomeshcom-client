package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/positions"
)

func TestImportNodesImportsPositionsJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := writeNodesJSON(t, map[string]positions.Record{
		"IU5PMP-1": nodeRecord(t),
	})

	if err := db.ImportNodes(ctx, path); err != nil {
		t.Fatalf("ImportNodes() error = %v", err)
	}

	var lat, lng float64
	var alt, rssi, snr int
	var hardwareID, firstSeen, lastSeen, lastDirectSeen, via string
	err := db.SQL().QueryRowContext(ctx, `
		SELECT lat, lng, alt, hw_id, firstseen, lastseen, lastdirectseen, rssi, snr, via
		FROM nodes WHERE node_id = 'IU5PMP-1'
	`).Scan(&lat, &lng, &alt, &hardwareID, &firstSeen, &lastSeen, &lastDirectSeen, &rssi, &snr, &via)
	if err != nil {
		t.Fatalf("query imported node: %v", err)
	}
	if lat != 43.7 || lng != 10.4 || alt != 120 || hardwareID != "LORA" || rssi != -70 || snr != 8 {
		t.Fatalf("imported node mismatch: lat=%v lng=%v alt=%d hw=%q rssi=%d snr=%d", lat, lng, alt, hardwareID, rssi, snr)
	}
	if !strings.HasPrefix(firstSeen, "2026-01-02T03:04:05Z") {
		t.Fatalf("firstseen = %q", firstSeen)
	}
	if !strings.HasPrefix(lastSeen, "2026-01-02T04:04:05Z") {
		t.Fatalf("lastseen = %q", lastSeen)
	}
	if !strings.HasPrefix(lastDirectSeen, "2026-01-02T05:04:05Z") {
		t.Fatalf("lastdirectseen = %q", lastDirectSeen)
	}
	if via != `["RELAY1","RELAY2"]` {
		t.Fatalf("via = %q", via)
	}
	assertImportRecorded(t, db.SQL(), nodesImportSource, 1)
}

func TestImportNodesTreatsMissingViaAsEmptyArray(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "positions.json")
	writeFile(t, path, `{"NODE-1":{"lat":43.7,"lng":10.4,"lastseen":"2026-07-01T12:00:00Z"}}`)

	if err := db.ImportNodes(ctx, path); err != nil {
		t.Fatalf("ImportNodes() error = %v", err)
	}

	var via string
	if err := db.SQL().QueryRowContext(ctx, `SELECT via FROM nodes WHERE node_id = 'NODE-1'`).Scan(&via); err != nil {
		t.Fatalf("query via: %v", err)
	}
	if via != `[]` {
		t.Fatalf("via = %q, want []", via)
	}
}

func TestImportNodesMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	path := filepath.Join(t.TempDir(), "missing.json")
	if err := db.ImportNodes(ctx, path); err != nil {
		t.Fatalf("ImportNodes() missing file error = %v", err)
	}

	assertImportRecorded(t, db.SQL(), nodesImportSource, 0)
}

func TestImportNodesInvalidJSONFailsWithoutImportRecord(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "positions.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := db.ImportNodes(ctx, path)
	if err == nil {
		t.Fatal("ImportNodes() error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decode nodes import") {
		t.Fatalf("ImportNodes() error = %v, want decode context", err)
	}

	var count int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM data_imports WHERE source = ?`, nodesImportSource).Scan(&count); err != nil {
		t.Fatalf("query data_imports: %v", err)
	}
	if count != 0 {
		t.Fatalf("data_imports count = %d, want 0", count)
	}
}

func TestImportNodesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := writeNodesJSON(t, map[string]positions.Record{
		"IU5PMP-1": nodeRecord(t),
	})

	if err := db.ImportNodes(ctx, path); err != nil {
		t.Fatalf("first ImportNodes() error = %v", err)
	}
	updated := nodeRecord(t)
	updated.Latitude = 99
	path = writeNodesJSON(t, map[string]positions.Record{"IU5PMP-1": updated})
	if err := db.ImportNodes(ctx, path); err != nil {
		t.Fatalf("second ImportNodes() error = %v", err)
	}

	var lat float64
	if err := db.SQL().QueryRowContext(ctx, `SELECT lat FROM nodes WHERE node_id = 'IU5PMP-1'`).Scan(&lat); err != nil {
		t.Fatalf("query imported node: %v", err)
	}
	if lat != 43.7 {
		t.Fatalf("lat = %v, want first import value", lat)
	}
	assertImportRecorded(t, db.SQL(), nodesImportSource, 1)
}

func writeNodesJSON(t *testing.T, records map[string]positions.Record) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "positions.json")
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func nodeRecord(t *testing.T) positions.Record {
	t.Helper()
	firstSeen := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	lastSeen := firstSeen.Add(time.Hour)
	lastDirectSeen := firstSeen.Add(2 * time.Hour)
	return positions.Record{
		Latitude:       43.7,
		Longitude:      10.4,
		Altitude:       120,
		HardwareID:     "LORA",
		FirstSeen:      firstSeen,
		LastSeen:       lastSeen,
		LastDirectSeen: &lastDirectSeen,
		RSSI:           -70,
		SNR:            8,
		Via:            []string{"RELAY1", "RELAY2"},
	}
}

func assertImportRecorded(t *testing.T, db *sql.DB, source string, wantCount int) {
	t.Helper()
	var count int
	var recordCount int
	err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(record_count), 0) FROM data_imports WHERE source = ?`, source).Scan(&count, &recordCount)
	if err != nil {
		t.Fatalf("query data_imports: %v", err)
	}
	if count != 1 || recordCount != wantCount {
		t.Fatalf("data_imports count=%d record_count=%d, want 1/%d", count, recordCount, wantCount)
	}
}
