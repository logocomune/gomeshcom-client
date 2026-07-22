package station_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/station"
)

func TestLoadLegacyUsesConfigDefault(t *testing.T) {
	got, err := station.LoadLegacy(station.DefaultPath(t.TempDir()), "IU5PMP-1")
	if err != nil {
		t.Fatalf("LoadLegacy() error = %v", err)
	}
	if got != "IU5PMP-1" {
		t.Fatalf("LoadLegacy() = %q, want IU5PMP-1", got)
	}
}

func TestLoadLegacyLoadsPersisted(t *testing.T) {
	path := station.DefaultPath(t.TempDir())
	writeLegacyStation(t, path, "QQ1ABC-1")

	got, err := station.LoadLegacy(path, "IU5PMP-1")
	if err != nil {
		t.Fatalf("LoadLegacy() error = %v", err)
	}
	if got != "QQ1ABC-1" {
		t.Fatalf("LoadLegacy() = %q, want QQ1ABC-1", got)
	}
}

func TestLoadLegacyIgnoresInvalidPersisted(t *testing.T) {
	path := station.DefaultPath(t.TempDir())
	writeLegacyStation(t, path, "!!")

	got, err := station.LoadLegacy(path, "IU5PMP-1")
	if err != nil {
		t.Fatalf("LoadLegacy() error = %v", err)
	}
	if got != "IU5PMP-1" {
		t.Fatalf("LoadLegacy() = %q, want fallback", got)
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

func TestInMemoryNoPersistence(t *testing.T) {
	id := station.NewInMemory("IU5PMP-1")

	if _, err := id.Update("QQ0QQ-2"); err != nil {
		t.Fatal(err)
	}
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
			_, _ = id.Update("QQ0QQ-2")
		}()
	}
	wg.Wait()
}

func writeLegacyStation(t *testing.T, path string, callsign string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{"callsign": callsign})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
