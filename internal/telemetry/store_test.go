package telemetry

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/storage"
)

func TestAppendStoresSamplesAndDirectSignal(t *testing.T) {
	ctx := context.Background()
	db := openTestStorage(t, ctx)
	store := NewSQLite(db.SQL())
	receivedAt := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	rssi := -102
	snr := 4

	err := store.Append(ctx, meshcom.Telemetry{
		Source:     "QQ1ABC-1",
		SourceType: "lora",
		RSSI:       &rssi,
		SNR:        &snr,
		Temp1:      floatPtr(21.5),
		Temp2:      floatPtr(11.25),
		Humidity:   floatPtr(55),
		Battery:    intPtr(90),
		QFE:        floatPtr(1004),
		QNH:        floatPtr(1013.2),
		Gas:        floatPtr(50000),
		CO2:        floatPtr(420),
	}, receivedAt)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	assertCount(t, db, "telemetry_samples", 8)
	assertCount(t, db, "telemetry_direct_signal", 1)

	var value float64
	if err := db.SQL().QueryRowContext(ctx, `
		SELECT value FROM telemetry_samples
		WHERE src_origin = 'QQ1ABC-1' AND metric = 'temp1'
	`).Scan(&value); err != nil {
		t.Fatalf("query temp1: %v", err)
	}
	if value != 21.5 {
		t.Fatalf("temp1 value = %v, want 21.5", value)
	}

	var gotRSSI, gotSNR int
	if err := db.SQL().QueryRowContext(ctx, `
		SELECT rssi, snr FROM telemetry_direct_signal WHERE src_origin = 'QQ1ABC-1'
	`).Scan(&gotRSSI, &gotSNR); err != nil {
		t.Fatalf("query direct signal: %v", err)
	}
	if gotRSSI != rssi || gotSNR != snr {
		t.Fatalf("signal = %d/%d, want %d/%d", gotRSSI, gotSNR, rssi, snr)
	}
}

func TestAppendSkipsDirectSignalForRelayedTelemetry(t *testing.T) {
	ctx := context.Background()
	db := openTestStorage(t, ctx)
	store := NewSQLite(db.SQL())
	rssi := -110

	err := store.Append(ctx, meshcom.Telemetry{
		Source:  "QQ1ABC-1,QQ1RLY-1",
		RSSI:    &rssi,
		Battery: intPtr(75),
	}, time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	assertCount(t, db, "telemetry_samples", 1)
	assertCount(t, db, "telemetry_direct_signal", 0)

	var origin string
	if err := db.SQL().QueryRowContext(ctx, `SELECT src_origin FROM telemetry_samples LIMIT 1`).Scan(&origin); err != nil {
		t.Fatalf("query origin: %v", err)
	}
	if origin != "QQ1ABC-1" {
		t.Fatalf("src_origin = %q, want QQ1ABC-1", origin)
	}
}

func TestAppendStoresExplicitZeroAndSkipsMissingMetrics(t *testing.T) {
	ctx := context.Background()
	db := openTestStorage(t, ctx)
	store := NewSQLite(db.SQL())

	err := store.Append(ctx, meshcom.Telemetry{
		Source: "QQ1ABC-1",
		Temp1:  floatPtr(0),
	}, time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	assertCount(t, db, "telemetry_samples", 1)

	var metric string
	var value float64
	if err := db.SQL().QueryRowContext(ctx, `SELECT metric, value FROM telemetry_samples`).Scan(&metric, &value); err != nil {
		t.Fatalf("query sample: %v", err)
	}
	if metric != "temp1" || value != 0 {
		t.Fatalf("sample = %s/%v, want temp1/0", metric, value)
	}
}

func openTestStorage(t *testing.T, ctx context.Context) *storage.DB {
	t.Helper()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertCount(t *testing.T, db *storage.DB, table string, want int) {
	t.Helper()
	var count int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	return &value
}
