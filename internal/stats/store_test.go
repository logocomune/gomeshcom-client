package stats

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"testing/quick"
	"time"
)

// ---- hourKey ----------------------------------------------------------------

func TestHourKeyTruncatesToHour(t *testing.T) {
	cases := []struct {
		in   time.Time
		want time.Time
	}{
		{
			time.Date(2024, 3, 15, 14, 37, 22, 999, time.UTC),
			time.Date(2024, 3, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		got := time.Unix(hourKey(tc.in), 0).UTC()
		if !got.Equal(tc.want) {
			t.Errorf("hourKey(%v) → %v, want %v", tc.in, got, tc.want)
		}
	}
}

// ---- RecordPacket / Bucket invariants ----------------------------------------

func TestRecordPacketTotalSumsKinds(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC()

	store.RecordPacket(KindDM, now, nil)
	store.RecordPacket(KindPublic, now, nil)
	store.RecordPacket(KindTelemetry, now, nil)
	d := 3.14
	store.RecordPacket(KindPosition, now, &d)
	store.RecordPacket(KindError, now, nil) // must NOT count in Total

	key := hourKey(now)
	store.mu.Lock()
	b := store.buckets[key]
	store.mu.Unlock()

	if b == nil {
		t.Fatal("no bucket created")
	}
	wantTotal := b.DM + b.Public + b.Telemetry + b.Position
	if b.Total != wantTotal {
		t.Errorf("Total=%d, want sum of kinds=%d", b.Total, wantTotal)
	}
	if b.Errors != 1 {
		t.Errorf("Errors=%d, want 1", b.Errors)
	}
	if len(b.DistanceKm) == 0 {
		t.Error("DistanceKm should be non-empty after position record with distance")
	}
}

// Property: Total == DM + Public + Telemetry + Position for any sequence.
func TestRecordPacketTotalProperty(t *testing.T) {
	kinds := []Kind{KindDM, KindPublic, KindTelemetry, KindPosition, KindError}
	f := func(indices []uint8) bool {
		store := newTestStore(t)
		now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		// recent enough to pass 30-day retention cutoff
		now = time.Now().UTC().Truncate(time.Hour)
		for _, idx := range indices {
			k := kinds[int(idx)%len(kinds)]
			store.RecordPacket(k, now, nil)
		}

		key := hourKey(now)
		store.mu.Lock()
		b := store.buckets[key]
		store.mu.Unlock()
		if b == nil {
			return true
		}
		return b.Total == b.DM+b.Public+b.Telemetry+b.Position
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 200, Rand: rand.New(rand.NewSource(1))}); err != nil {
		t.Fatal(err)
	}
}

// ---- save / Load round-trip -------------------------------------------------

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour)

	store.RecordPacket(KindDM, now, nil)
	store.RecordPacket(KindPublic, now, nil)
	dist := 12.5
	store.RecordPacket(KindPosition, now, &dist)
	store.RecordDMAck(now)

	if err := store.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file exists and contains expected JSON.
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var raw map[int64]*Bucket
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	key := hourKey(now)
	b, ok := raw[key]
	if !ok {
		t.Fatal("bucket missing from JSON file")
	}
	if b.DM != 1 || b.Public != 1 || b.Position != 1 || b.DMAck != 1 {
		t.Errorf("unexpected counts: %+v", b)
	}

	// Load into fresh store and verify.
	store2 := New(Config{Path: store.path, RetentionDays: 30})
	if err := store2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	store2.mu.Lock()
	loaded := store2.buckets[key]
	store2.mu.Unlock()
	if loaded == nil {
		t.Fatal("bucket not loaded")
	}
	if loaded.DM != 1 || loaded.Public != 1 || loaded.DMAck != 1 {
		t.Errorf("loaded counts: %+v", loaded)
	}
}

// Property: marshal/unmarshal round-trip preserves scalar fields.
func TestBucketMarshalRoundTrip(t *testing.T) {
	f := func(dm, pub, tele, pos, acks, errs, total int8) bool {
		orig := Bucket{
			HourUnix:  1700000000,
			DM:        int(dm),
			Public:    int(pub),
			Telemetry: int(tele),
			Position:  int(pos),
			DMAck:     int(acks),
			Errors:    int(errs),
			Total:     int(total),
		}
		data, err := json.Marshal(orig)
		if err != nil {
			return false
		}
		var got Bucket
		if err := json.Unmarshal(data, &got); err != nil {
			return false
		}
		return got.HourUnix == orig.HourUnix &&
			got.DM == orig.DM &&
			got.Public == orig.Public &&
			got.Telemetry == orig.Telemetry &&
			got.Position == orig.Position &&
			got.DMAck == orig.DMAck &&
			got.Errors == orig.Errors &&
			got.Total == orig.Total
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 500, Rand: rand.New(rand.NewSource(2))}); err != nil {
		t.Fatal(err)
	}
}

// ---- prune ------------------------------------------------------------------

func TestPruneRemovesOldBuckets(t *testing.T) {
	store := newTestStore(t)

	old := time.Now().UTC().AddDate(0, 0, -4) // 4 days ago, older than 3-day retention
	store.mu.Lock()
	store.buckets[hourKey(old)] = &Bucket{HourUnix: hourKey(old), DM: 99}
	store.mu.Unlock()

	recent := time.Now().UTC()
	store.RecordPacket(KindPublic, recent, nil)

	store.pruneExpired(time.Now().UTC())

	store.mu.Lock()
	_, hasOld := store.buckets[hourKey(old)]
	_, hasRecent := store.buckets[hourKey(recent)]
	store.mu.Unlock()

	if hasOld {
		t.Error("old bucket should have been pruned")
	}
	if !hasRecent {
		t.Error("recent bucket should remain")
	}
}

// ---- ReadRange --------------------------------------------------------------

func TestReadRange(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)

	for i := 0; i < 3; i++ {
		store.RecordPacket(KindPublic, base.Add(time.Duration(i)*time.Hour), nil)
	}

	from := base.Add(-1 * time.Hour)
	to := base.Add(4 * time.Hour)
	buckets, err := store.ReadRange(from, to)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(buckets) != 3 {
		t.Errorf("got %d buckets, want 3", len(buckets))
	}
	if !sort.SliceIsSorted(buckets, func(i, j int) bool {
		return buckets[i].HourUnix < buckets[j].HourUnix
	}) {
		t.Error("buckets not sorted by hour")
	}
}

// ---- helpers ----------------------------------------------------------------

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stats.json")
	return New(Config{Path: path, RetentionDays: 3})
}
