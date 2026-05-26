package chatstatus

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

var t0 = time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
var t1 = time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)

func TestRecordIncoming(t *testing.T) {
	tests := []struct {
		name        string
		calls       int
		wantUnread  int
		wantDirty   bool
	}{
		{"single", 1, 1, true},
		{"multiple", 5, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
			if err != nil {
				t.Fatal(err)
			}

			for i := 0; i < tt.calls; i++ {
				s.RecordIncoming("P_broadcast", t0, "hello")
			}

			snap := s.Snapshot()
			e, ok := snap["P_broadcast"]
			if !ok {
				t.Fatal("expected P_broadcast in snapshot")
			}
			if e.UnreadCount != tt.wantUnread {
				t.Fatalf("UnreadCount = %d, want %d", e.UnreadCount, tt.wantUnread)
			}
			if !e.LastMsgReceived.Equal(t0) {
				t.Fatalf("LastMsgReceived = %v, want %v", e.LastMsgReceived, t0)
			}

			s.mu.Lock()
			dirty := s.dirty
			s.mu.Unlock()
			if dirty != tt.wantDirty {
				t.Fatalf("dirty = %v, want %v", dirty, tt.wantDirty)
			}
		})
	}
}

func TestMarkRead(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
	if err != nil {
		t.Fatal(err)
	}

	s.RecordIncoming("P_broadcast", t0, "hello")
	s.RecordIncoming("P_broadcast", t0, "hello")
	s.MarkRead("P_broadcast", t1)

	snap := s.Snapshot()
	e := snap["P_broadcast"]
	if e.UnreadCount != 0 {
		t.Fatalf("UnreadCount after MarkRead = %d, want 0", e.UnreadCount)
	}
	if !e.LastRead.Equal(t1) {
		t.Fatalf("LastRead = %v, want %v", e.LastRead, t1)
	}

	s.mu.Lock()
	dirty := s.dirty
	s.mu.Unlock()
	if !dirty {
		t.Fatal("expected dirty=true after MarkRead")
	}
}

func TestSnapshotIsImmutable(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
	if err != nil {
		t.Fatal(err)
	}

	s.RecordIncoming("P_broadcast", t0, "hello")
	snap1 := s.Snapshot()

	s.RecordIncoming("P_broadcast", t0, "hello")
	snap2 := s.Snapshot()

	if snap1["P_broadcast"].UnreadCount != 1 {
		t.Fatalf("snap1 mutated: UnreadCount = %d", snap1["P_broadcast"].UnreadCount)
	}
	if snap2["P_broadcast"].UnreadCount != 2 {
		t.Fatalf("snap2 wrong: UnreadCount = %d", snap2["P_broadcast"].UnreadCount)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "msg_idx.json")

	s1, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	s1.RecordIncoming("P_broadcast", t0, "first")
	s1.RecordIncoming("P_broadcast", t0, "second")
	s1.MarkRead("DM_QQ1ABC-1", t1)
	s1.RecordIncoming("P_1", t0, "hello")

	if err := s1.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("msg_idx.json not created")
	}

	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	snap := s2.Snapshot()

	if snap["P_broadcast"].UnreadCount != 2 {
		t.Fatalf("P_broadcast UnreadCount = %d, want 2", snap["P_broadcast"].UnreadCount)
	}
	if !snap["P_broadcast"].LastMsgReceived.Equal(t0) {
		t.Fatalf("P_broadcast LastMsgReceived = %v, want %v", snap["P_broadcast"].LastMsgReceived, t0)
	}
	if snap["DM_QQ1ABC-1"].UnreadCount != 0 {
		t.Fatalf("DM_QQ1ABC-1 UnreadCount = %d, want 0", snap["DM_QQ1ABC-1"].UnreadCount)
	}
	if !snap["DM_QQ1ABC-1"].LastRead.Equal(t1) {
		t.Fatalf("DM_QQ1ABC-1 LastRead = %v, want %v", snap["DM_QQ1ABC-1"].LastRead, t1)
	}
	if snap["P_1"].UnreadCount != 1 {
		t.Fatalf("P_1 UnreadCount = %d, want 1", snap["P_1"].UnreadCount)
	}
	if snap["P_broadcast"].LastMsg != "second" {
		t.Fatalf("P_broadcast LastMsg = %q, want second", snap["P_broadcast"].LastMsg)
	}
}

func TestRemove(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
	if err != nil {
		t.Fatal(err)
	}

	s.RecordIncoming("P_broadcast", t0, "hello")
	s.RecordIncoming("DM_QQ1ABC-1", t0, "hello")

	s.Remove("P_broadcast")

	snap := s.Snapshot()
	if _, present := snap["P_broadcast"]; present {
		t.Fatal("P_broadcast must be absent after Remove")
	}
	if _, present := snap["DM_QQ1ABC-1"]; !present {
		t.Fatal("DM_QQ1ABC-1 must remain after removing P_broadcast")
	}

	s.mu.Lock()
	dirty := s.dirty
	s.mu.Unlock()
	if !dirty {
		t.Fatal("expected dirty=true after Remove")
	}
}

func TestRemoveNonExistentIsNoop(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic or set dirty.
	s.Remove("P_broadcast")

	s.mu.Lock()
	dirty := s.dirty
	s.mu.Unlock()
	if dirty {
		t.Fatal("Remove on missing entry must not set dirty")
	}
}

func TestSaveIfDirtyNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "msg_idx.json")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	// No mutations — SaveIfDirty must not create the file.
	if err := s.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not be created when dirty=false")
	}
}

func TestLoadCleansTmpFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "msg_idx.json")
	// Simulate a leftover .tmp from a previous crash.
	if err := os.WriteFile(path+".tmp", []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp file should be removed on Load")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 10
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				s.RecordIncoming("P_broadcast", t0, "hello")
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				s.MarkRead("P_broadcast", t1)
			}
		}()
	}

	wg.Wait()
	// Must not panic or race.
}

func TestPropertyMarkReadZeroesUnread(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
		if err != nil {
			rt.Fatal(err)
		}

		incoming := rapid.IntRange(0, 20).Draw(rt, "incoming")
		for i := 0; i < incoming; i++ {
			s.RecordIncoming("P_broadcast", t0, "hello")
		}

		s.MarkRead("P_broadcast", t1)

		snap := s.Snapshot()
		if snap["P_broadcast"].UnreadCount != 0 {
			rt.Fatalf("after MarkRead UnreadCount = %d, want 0", snap["P_broadcast"].UnreadCount)
		}
	})
}

func TestPropertyUnreadNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		s, err := New(filepath.Join(t.TempDir(), "msg_idx.json"))
		if err != nil {
			rt.Fatal(err)
		}

		ops := rapid.SliceOf(rapid.Bool()).Draw(rt, "ops")
		for _, isIncoming := range ops {
			if isIncoming {
				s.RecordIncoming("P_broadcast", t0, "hello")
			} else {
				s.MarkRead("P_broadcast", t1)
			}
		}

		snap := s.Snapshot()
		if snap["P_broadcast"].UnreadCount < 0 {
			rt.Fatalf("UnreadCount negative: %d", snap["P_broadcast"].UnreadCount)
		}
	})
}
