package positions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
)

func TestStoreUpdatesPositionByOriginCallsign(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "positions.json"))
	firstSeen := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lastSeen := firstSeen.Add(time.Minute)

	created := store.Update(meshcom.Position{
		Source:    "QQ5EKX-11,QQ5AKT-10",
		Latitude:  43.5,
		Longitude: 10.3,
		Altitude:  367,
		RSSI:      -108,
		SNR:       1,
	}, firstSeen)
	updated := store.Update(meshcom.Position{
		Source:    "QQ5EKX-11,QQ5AKT-10",
		Latitude:  43.6,
		Longitude: 10.4,
		Altitude:  380,
		RSSI:      -100,
		SNR:       3,
	}, lastSeen)

	if !created || !updated {
		t.Fatalf("updates = %t, %t; want both true", created, updated)
	}

	record := store.Snapshot()["QQ5EKX-11"]
	if record.FirstSeen != firstSeen {
		t.Fatalf("first seen = %s, want %s", record.FirstSeen, firstSeen)
	}
	if record.LastSeen != lastSeen {
		t.Fatalf("last seen = %s, want %s", record.LastSeen, lastSeen)
	}
	if record.Latitude != 43.6 || record.Longitude != 10.4 || record.Altitude != 380 {
		t.Fatalf("record position = %+v", record)
	}
	// indirect: rssi/snr must NOT be overwritten on origin
	if record.RSSI != 0 || record.SNR != 0 {
		t.Fatalf("indirect origin must keep zero rssi/snr, got RSSI=%d SNR=%d", record.RSSI, record.SNR)
	}
	if !slices.Equal(record.Via, []string{"QQ5AKT-10"}) {
		t.Fatalf("record via = %v, want QQ5AKT-10", record.Via)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes", "positions.json")
	seenAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	store := New(path)

	store.Update(meshcom.Position{
		Source:     "QQ1ABC-1",
		Latitude:   48.1,
		Longitude:  16.3,
		Altitude:   123,
		HardwareID: "TLORA_V2",
		RSSI:       -90,
		SNR:        8,
	}, seenAt)

	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read positions file: %v", err)
	}
	var raw map[string]Record
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal positions file: %v", err)
	}
	if _, ok := raw["QQ1ABC-1"]; !ok {
		t.Fatalf("saved keys = %v, want QQ1ABC-1", raw)
	}
	if raw["QQ1ABC-1"].LastDirectSeen == nil || !raw["QQ1ABC-1"].LastDirectSeen.Equal(seenAt) {
		t.Fatalf("saved lastdirectseen = %v, want %v", raw["QQ1ABC-1"].LastDirectSeen, seenAt)
	}

	loaded := New(path)
	if err := loaded.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := loaded.Snapshot()["QQ1ABC-1"]; !recordsEqual(got, raw["QQ1ABC-1"]) {
		t.Fatalf("loaded = %+v, want %+v", got, raw["QQ1ABC-1"])
	}
}

func TestStoreLoadErrors(t *testing.T) {
	dir := t.TempDir()

	// Corrupt JSON
	path1 := filepath.Join(dir, "corrupt.json")
	os.WriteFile(path1, []byte("{"), 0644)
	s1 := New(path1)
	if err := s1.Load(); err == nil {
		t.Error("load corrupt json: want error, got nil")
	}

	// Missing file is NOT an error
	path2 := filepath.Join(dir, "missing.json")
	s2 := New(path2)
	if err := s2.Load(); err != nil {
		t.Errorf("load missing file: want nil, got %v", err)
	}
}

func TestStoreSaveIfDirtyNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "positions.json")
	store := New(path)

	if err := store.SaveIfDirty(); err != nil {
		t.Errorf("SaveIfDirty empty: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("SaveIfDirty empty created file")
	}
}

func TestStoreStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "positions.json")
	store := New(path)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go store.Start(ctx)

	store.Update(meshcom.Position{Source: "QQ0QQ-1"}, time.Now())

	cancel() // Should trigger save on shutdown

	// Wait a bit for save to complete
	time.Sleep(50 * time.Millisecond)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not saved on context cancel: %v", err)
	}
}

func recordsEqual(left Record, right Record) bool {
	directEqual := (left.LastDirectSeen == nil) == (right.LastDirectSeen == nil) &&
		(left.LastDirectSeen == nil || left.LastDirectSeen.Equal(*right.LastDirectSeen))
	return left.Latitude == right.Latitude &&
		left.Longitude == right.Longitude &&
		left.Altitude == right.Altitude &&
		left.HardwareID == right.HardwareID &&
		left.FirstSeen.Equal(right.FirstSeen) &&
		left.LastSeen.Equal(right.LastSeen) &&
		left.RSSI == right.RSSI &&
		left.SNR == right.SNR &&
		slices.Equal(left.Via, right.Via) &&
		directEqual
}

func TestLastDirectSeen(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)
	t2 := t1.Add(10 * time.Minute)
	pos := func(src string) meshcom.Position {
		return meshcom.Position{Source: src, Latitude: 43.0, Longitude: 11.0}
	}

	// direct packet → LastDirectSeen set
	s := New(filepath.Join(dir, "p1.json"))
	s.Update(pos("QQ0QQ-1"), t0)
	if r := s.Snapshot()["QQ0QQ-1"]; r.LastDirectSeen == nil || !r.LastDirectSeen.Equal(t0) {
		t.Fatalf("direct: LastDirectSeen = %v, want %v", r.LastDirectSeen, t0)
	}

	// relay packet → LastDirectSeen nil on origin
	s = New(filepath.Join(dir, "p2.json"))
	s.Update(pos("QQ0QQ-1,RELAY"), t0)
	if r := s.Snapshot()["QQ0QQ-1"]; r.LastDirectSeen != nil {
		t.Fatalf("relay-only: LastDirectSeen = %v, want nil", r.LastDirectSeen)
	}

	// relay after direct → preserves origin LastDirectSeen
	s = New(filepath.Join(dir, "p3.json"))
	s.Update(pos("QQ0QQ-1"), t0)
	s.Update(pos("QQ0QQ-1,RELAY"), t1)
	if r := s.Snapshot()["QQ0QQ-1"]; r.LastDirectSeen == nil || !r.LastDirectSeen.Equal(t0) {
		t.Fatalf("relay-after-direct: LastDirectSeen = %v, want %v", r.LastDirectSeen, t0)
	}

	// direct after relay → updates origin LastDirectSeen
	s = New(filepath.Join(dir, "p4.json"))
	s.Update(pos("QQ0QQ-1,RELAY"), t0)
	s.Update(pos("QQ0QQ-1"), t2)
	if r := s.Snapshot()["QQ0QQ-1"]; r.LastDirectSeen == nil || !r.LastDirectSeen.Equal(t2) {
		t.Fatalf("direct-after-relay: LastDirectSeen = %v, want %v", r.LastDirectSeen, t2)
	}
}

func TestLastHopFreshnessOnPosPacket(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)

	// Pre-populate relay node with a pos packet so it has a record.
	s := New(filepath.Join(dir, "p.json"))
	s.Update(meshcom.Position{Source: "QQ0REL-1", Latitude: 44.0, Longitude: 11.0}, t0)

	// Indirect pos from origin via RELAY-1 → RELAY-1 should get direct freshness.
	s.Update(meshcom.Position{
		Source:    "QQ0ORG-1,QQ0REL-1",
		Latitude:  43.0,
		Longitude: 10.0,
		RSSI:      -90,
		SNR:       5,
	}, t1)

	relay := s.Snapshot()["QQ0REL-1"]
	if relay.LastDirectSeen == nil || !relay.LastDirectSeen.Equal(t1) {
		t.Fatalf("relay LastDirectSeen = %v, want %v", relay.LastDirectSeen, t1)
	}
	if relay.RSSI != -90 || relay.SNR != 5 {
		t.Fatalf("relay RSSI/SNR = %d/%d, want -90/5", relay.RSSI, relay.SNR)
	}

	// Origin must NOT get rssi/snr updated (indirect).
	origin := s.Snapshot()["QQ0ORG-1"]
	if origin.RSSI != 0 || origin.SNR != 0 {
		t.Fatalf("indirect origin RSSI/SNR = %d/%d, want 0/0", origin.RSSI, origin.SNR)
	}
	if origin.LastDirectSeen != nil {
		t.Fatalf("indirect origin LastDirectSeen = %v, want nil", origin.LastDirectSeen)
	}
}

func TestViaChainFreshnessOnPosPacket(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)

	s := New(filepath.Join(dir, "p.json"))
	s.Update(meshcom.Position{Source: "QQ0REL-1", Latitude: 44.0, Longitude: 11.0}, t0)
	s.Update(meshcom.Position{Source: "QQ0MID-1", Latitude: 45.0, Longitude: 12.0}, t0)

	s.Update(meshcom.Position{
		Source:    "QQ0ORG-1,QQ0MID-1,QQ0REL-1",
		Latitude:  43.0,
		Longitude: 10.0,
		RSSI:      -90,
		SNR:       5,
	}, t1)

	relay := s.Snapshot()["QQ0REL-1"]
	if relay.LastSeen != t1 || relay.LastDirectSeen == nil || !relay.LastDirectSeen.Equal(t1) {
		t.Fatalf("relay freshness = %+v, want lastSeen/lastDirectSeen = %v", relay, t1)
	}
	if relay.RSSI != -90 || relay.SNR != 5 {
		t.Fatalf("relay RSSI/SNR = %d/%d, want -90/5", relay.RSSI, relay.SNR)
	}

	mid := s.Snapshot()["QQ0MID-1"]
	if mid.LastSeen != t1 {
		t.Fatalf("mid LastSeen = %v, want %v", mid.LastSeen, t1)
	}
	if mid.LastDirectSeen == nil || !mid.LastDirectSeen.Equal(t0) {
		t.Fatalf("mid LastDirectSeen = %v, want %v", mid.LastDirectSeen, t0)
	}
	if mid.RSSI != 0 || mid.SNR != 0 {
		t.Fatalf("mid RSSI/SNR = %d/%d, want 0/0", mid.RSSI, mid.SNR)
	}

	origin := s.Snapshot()["QQ0ORG-1"]
	if origin.LastSeen != t1 {
		t.Fatalf("origin LastSeen = %v, want %v", origin.LastSeen, t1)
	}
	if origin.LastDirectSeen != nil {
		t.Fatalf("origin LastDirectSeen = %v, want nil", origin.LastDirectSeen)
	}
}

func TestLastHopFreshnessSkippedIfNoRecord(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	// Relay has no pre-existing record → should be skipped (not created).
	s := New(filepath.Join(dir, "p.json"))
	s.Update(meshcom.Position{
		Source:    "ORIGIN-1,GHOST-RELAY",
		Latitude:  43.0,
		Longitude: 10.0,
		RSSI:      -90,
		SNR:       5,
	}, t0)

	snap := s.Snapshot()
	if _, exists := snap["GHOST-RELAY"]; exists {
		t.Fatal("relay with no pre-existing record must not be created")
	}
}

func TestTouchFromPacketDirect(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Minute)

	s := New(filepath.Join(dir, "p.json"))
	// Create initial record via pos packet.
	s.Update(meshcom.Position{Source: "A-1", Latitude: 43.0, Longitude: 10.0}, t0)

	// Touch via direct msg packet.
	changed := s.TouchFromPacket("A-1", -80, 7, t1)
	if !changed {
		t.Fatal("TouchFromPacket: expected changed=true")
	}

	r := s.Snapshot()["A-1"]
	if r.LastSeen != t1 {
		t.Fatalf("LastSeen = %v, want %v", r.LastSeen, t1)
	}
	if r.LastDirectSeen == nil || !r.LastDirectSeen.Equal(t1) {
		t.Fatalf("LastDirectSeen = %v, want %v", r.LastDirectSeen, t1)
	}
	if r.RSSI != -80 || r.SNR != 7 {
		t.Fatalf("RSSI/SNR = %d/%d, want -80/7", r.RSSI, r.SNR)
	}
}

func TestTouchFromPacketDirectSkipsIfNoRecord(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	s := New(filepath.Join(dir, "p.json"))
	changed := s.TouchFromPacket("GHOST-1", -80, 7, t0)
	if changed {
		t.Fatal("TouchFromPacket on missing origin: expected changed=false")
	}
	if _, exists := s.Snapshot()["GHOST-1"]; exists {
		t.Fatal("TouchFromPacket must not create records")
	}
}

func TestTouchFromPacketIndirect(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Minute)

	s := New(filepath.Join(dir, "p.json"))
	// Create origin via an indirect pos so it has no lastDirectSeen.
	s.Update(meshcom.Position{Source: "ORIGIN-1,BOOTSTRAP", Latitude: 43.0, Longitude: 10.0}, t0)
	// Create relay via a direct pos so it has a pre-existing lastDirectSeen at t0.
	s.Update(meshcom.Position{Source: "RELAY-1", Latitude: 44.0, Longitude: 11.0, RSSI: -70, SNR: 2}, t0)

	// Indirect msg from ORIGIN-1 via RELAY-1.
	changed := s.TouchFromPacket("ORIGIN-1,RELAY-1", -95, 3, t1)
	if !changed {
		t.Fatal("TouchFromPacket indirect: expected changed=true")
	}

	origin := s.Snapshot()["ORIGIN-1"]
	if origin.LastSeen != t1 {
		t.Fatalf("origin LastSeen = %v, want %v", origin.LastSeen, t1)
	}
	// Indirect touch must NOT set lastDirectSeen (origin had none).
	if origin.LastDirectSeen != nil {
		t.Fatalf("origin LastDirectSeen = %v, want nil (indirect with no prior direct)", origin.LastDirectSeen)
	}
	// Indirect touch must NOT update rssi/snr on origin.
	if origin.RSSI != 0 || origin.SNR != 0 {
		t.Fatalf("origin RSSI/SNR = %d/%d, want 0/0 (indirect)", origin.RSSI, origin.SNR)
	}

	relay := s.Snapshot()["RELAY-1"]
	if relay.LastSeen != t1 {
		t.Fatalf("relay LastSeen = %v, want %v", relay.LastSeen, t1)
	}
	if relay.LastDirectSeen == nil || !relay.LastDirectSeen.Equal(t1) {
		t.Fatalf("relay LastDirectSeen = %v, want %v", relay.LastDirectSeen, t1)
	}
	if relay.RSSI != -95 || relay.SNR != 3 {
		t.Fatalf("relay RSSI/SNR = %d/%d, want -95/3", relay.RSSI, relay.SNR)
	}
}

func TestTouchFromPacketIndirectUpdatesAllViaHopsLastSeen(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Minute)

	s := New(filepath.Join(dir, "p.json"))
	s.Update(meshcom.Position{Source: "ORIGIN-1", Latitude: 43.0, Longitude: 10.0}, t0)
	s.Update(meshcom.Position{Source: "MID-1", Latitude: 44.0, Longitude: 11.0}, t0)
	s.Update(meshcom.Position{Source: "RELAY-1", Latitude: 45.0, Longitude: 12.0}, t0)

	changed := s.TouchFromPacket("ORIGIN-1,MID-1,RELAY-1", -95, 3, t1)
	if !changed {
		t.Fatal("TouchFromPacket indirect multi-hop: expected changed=true")
	}

	origin := s.Snapshot()["ORIGIN-1"]
	if origin.LastSeen != t1 || origin.LastDirectSeen == nil || !origin.LastDirectSeen.Equal(t0) || origin.RSSI != 0 || origin.SNR != 0 {
		t.Fatalf("origin freshness = %+v", origin)
	}

	mid := s.Snapshot()["MID-1"]
	if mid.LastSeen != t1 {
		t.Fatalf("mid LastSeen = %v, want %v", mid.LastSeen, t1)
	}
	if mid.LastDirectSeen == nil || !mid.LastDirectSeen.Equal(t0) {
		t.Fatalf("mid LastDirectSeen = %v, want %v", mid.LastDirectSeen, t0)
	}
	if mid.RSSI != 0 || mid.SNR != 0 {
		t.Fatalf("mid RSSI/SNR = %d/%d, want 0/0", mid.RSSI, mid.SNR)
	}

	relay := s.Snapshot()["RELAY-1"]
	if relay.LastSeen != t1 {
		t.Fatalf("relay LastSeen = %v, want %v", relay.LastSeen, t1)
	}
	if relay.LastDirectSeen == nil || !relay.LastDirectSeen.Equal(t1) {
		t.Fatalf("relay LastDirectSeen = %v, want %v", relay.LastDirectSeen, t1)
	}
	if relay.RSSI != -95 || relay.SNR != 3 {
		t.Fatalf("relay RSSI/SNR = %d/%d, want -95/3", relay.RSSI, relay.SNR)
	}
}

func TestTouchFromPacketIndirectSkipsRelayIfNoRecord(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(3 * time.Minute)

	s := New(filepath.Join(dir, "p.json"))
	// Only origin exists; relay has no record.
	s.Update(meshcom.Position{Source: "ORIGIN-1", Latitude: 43.0, Longitude: 10.0}, t0)

	s.TouchFromPacket("ORIGIN-1,GHOST-RELAY", -95, 3, t1)

	if _, exists := s.Snapshot()["GHOST-RELAY"]; exists {
		t.Fatal("relay with no record must not be created by TouchFromPacket")
	}
	// Origin lastSeen must still be updated.
	if s.Snapshot()["ORIGIN-1"].LastSeen != t1 {
		t.Fatalf("origin LastSeen not updated when relay skipped")
	}
}
