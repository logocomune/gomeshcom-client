package channelshow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/quick"
)

func TestStoreDefaultSnapshot(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "channel_show.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got := store.Snapshot()
	want := Config{Mode: ModeAll, Channels: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot = %+v, want %+v", got, want)
	}
}

func TestUpdateNormalizesAllowlist(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "channel_show.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := store.Update(Config{
		Mode:     " ALLOWLIST ",
		Channels: []string{" * ", "222", "222", "22201"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	want := Config{Mode: ModeAllowlist, Channels: []string{"*", "222", "22201"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Update = %+v, want %+v", got, want)
	}
}

func TestUpdateRejectsInvalidInput(t *testing.T) {
	tests := map[string]Config{
		"mode":          {Mode: "hidden", Channels: []string{"222"}},
		"empty channel": {Mode: ModeAllowlist, Channels: []string{""}},
		"letters":       {Mode: ModeAllowlist, Channels: []string{"abc"}},
		"mixed":         {Mode: ModeAllowlist, Channels: []string{"22a"}},
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			store, err := New(filepath.Join(t.TempDir(), "channel_show.json"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			if _, err := store.Update(input); err == nil {
				t.Fatal("Update error = nil, want validation error")
			}
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "channel_show.json")
	store, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	want, err := store.Update(Config{Mode: ModeAllowlist, Channels: []string{"*", "9", "22201"}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}

	loaded, err := New(path)
	if err != nil {
		t.Fatalf("New loaded: %v", err)
	}
	got := loaded.Snapshot()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded Snapshot = %+v, want %+v", got, want)
	}
}

func TestSaveIfDirtyNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "channel_show.json")
	store, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := store.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file exists after noop SaveIfDirty: %v", err)
	}
}

func TestSnapshotIsImmutable(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "channel_show.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := store.Update(Config{Mode: ModeAllowlist, Channels: []string{"222"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	snapshot := store.Snapshot()
	snapshot.Channels[0] = "999"

	got := store.Snapshot()
	if got.Channels[0] != "222" {
		t.Fatalf("store mutated through snapshot: %+v", got)
	}
}

func TestLoadCleansTmpFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channel_show.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(`{"mode":"allowlist","channels":["999"]}`), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"mode":"allowlist","channels":["222"]}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("tmp file still exists: %v", err)
	}
	if got := store.Snapshot().Channels; !reflect.DeepEqual(got, []string{"222"}) {
		t.Fatalf("channels = %+v, want [222]", got)
	}
}

func TestStartFlushesOnCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "channel_show.json")
	store, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := store.Update(Config{Mode: ModeAllowlist, Channels: []string{"222"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		store.Start(ctx)
		close(done)
	}()
	cancel()
	<-done

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(content), `"222"`) {
		t.Fatalf("saved content = %s, want channel 222", content)
	}
}

func TestConcurrentAccess(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "channel_show.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Update(Config{Mode: ModeAllowlist, Channels: []string{"222", "*"}}); err != nil {
				t.Errorf("Update: %v", err)
			}
			_ = store.Snapshot()
		}()
	}
	wg.Wait()
}

func TestPropertyNormalizeConfigIdempotent(t *testing.T) {
	property := func(rawMode string, rawChannels []string) bool {
		config, err := Normalize(Config{Mode: rawMode, Channels: rawChannels})
		if err != nil {
			return true
		}
		again, err := Normalize(config)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(config, again)
	}

	if err := quick.Check(property, nil); err != nil {
		t.Fatal(err)
	}
}

func FuzzNormalize(f *testing.F) {
	for _, seed := range []struct {
		mode     string
		channels string
	}{
		{ModeAll, ""},
		{ModeAllowlist, "*,222,22201"},
		{"", "222"},
		{"hidden", "abc"},
	} {
		f.Add(seed.mode, seed.channels)
	}

	f.Fuzz(func(t *testing.T, mode string, csv string) {
		channels := strings.Split(csv, ",")
		if csv == "" {
			channels = nil
		}
		config, err := Normalize(Config{Mode: mode, Channels: channels})
		if err != nil {
			return
		}
		payload, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var decoded Config
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		again, err := Normalize(decoded)
		if err != nil {
			t.Fatalf("Normalize decoded: %v", err)
		}
		if !reflect.DeepEqual(config, again) {
			t.Fatalf("round trip = %+v, want %+v", again, config)
		}
	})
}
