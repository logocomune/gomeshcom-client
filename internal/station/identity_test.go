package station_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/station"
)

func TestNewUsesConfigDefault(t *testing.T) {
	dir := t.TempDir()
	id, err := station.New(station.DefaultPath(dir), "IU5PMP-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := id.Current(); got != "IU5PMP-1" {
		t.Errorf("Current() = %q, want IU5PMP-1", got)
	}
}

func TestNewLoadsPersisted(t *testing.T) {
	dir := t.TempDir()
	path := station.DefaultPath(dir)

	// Pre-write a station.json.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"callsign": "QQ1ABC-1"})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := station.New(path, "IU5PMP-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Persisted value must win over fallback.
	if got := id.Current(); got != "QQ1ABC-1" {
		t.Errorf("Current() = %q, want QQ1ABC-1", got)
	}
}

func TestNewIgnoresInvalidPersisted(t *testing.T) {
	dir := t.TempDir()
	path := station.DefaultPath(dir)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write an invalid callsign; should fall back to config default.
	data, _ := json.Marshal(map[string]string{"callsign": "!!"})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := station.New(path, "IU5PMP-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := id.Current(); got != "IU5PMP-1" {
		t.Errorf("Current() = %q, want IU5PMP-1 (fallback)", got)
	}
}

func TestUpdateAcceptsValidCallsign(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	got, err := id.Update("QQ0QQ-2")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got != "QQ0QQ-2" {
		t.Errorf("returned %q, want QQ0QQ-2", got)
	}
	if id.Current() != "QQ0QQ-2" {
		t.Errorf("Current() = %q after update", id.Current())
	}
}

func TestUpdateNormalizesInput(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	got, err := id.Update(" iu5pmp-2 ")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got != "IU5PMP-2" {
		t.Errorf("returned %q, want IU5PMP-2", got)
	}
}

func TestUpdateRejectsInvalidKeepsPrevious(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	_, err := id.Update("!!")
	if err == nil {
		t.Fatal("expected error for invalid callsign")
	}
	if id.Current() != "IU5PMP-1" {
		t.Errorf("Current() = %q after rejected update, want IU5PMP-1", id.Current())
	}
}

func TestUpdateNoOpWhenEqual(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	got, err := id.Update("IU5PMP-1")
	if err != nil {
		t.Fatalf("Update no-op: %v", err)
	}
	if got != "IU5PMP-1" {
		t.Errorf("returned %q, want IU5PMP-1", got)
	}
}

func TestSaveIfDirtyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := station.DefaultPath(dir)

	id, err := station.New(path, "IU5PMP-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := id.Update("QQ0QQ-2"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := id.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}

	// Reload from file — must reflect the new callsign.
	id2, err := station.New(path, "FALLBACK-1")
	if err != nil {
		t.Fatalf("New reload: %v", err)
	}
	if id2.Current() != "QQ0QQ-2" {
		t.Errorf("reloaded = %q, want QQ0QQ-2", id2.Current())
	}
}

func TestSaveIfDirtyNoOpWhenClean(t *testing.T) {
	dir := t.TempDir()
	path := station.DefaultPath(dir)

	id, err := station.New(path, "IU5PMP-1")
	if err != nil {
		t.Fatal(err)
	}
	// No Update called — SaveIfDirty must not create the file.
	if err := id.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("station.json must not be created before any update")
	}
}

func TestInMemoryNoPersistence(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	if _, err := id.Update("QQ0QQ-2"); err != nil {
		t.Fatal(err)
	}
	// SaveIfDirty on in-memory must never error.
	if err := id.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty on in-memory: %v", err)
	}
}

func TestConcurrentCurrentUpdate(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = id.Current()
		}()
		go func() {
			defer wg.Done()
			// Mix of valid and invalid updates — must not panic or race.
			_, _ = id.Update("QQ0QQ-2")
		}()
	}
	wg.Wait()
}
