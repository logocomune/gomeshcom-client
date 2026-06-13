package legacymigrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---- helpers ----------------------------------------------------------------

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(v); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

func writeJSONL(t *testing.T, path string, records []jsonRecord) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode record: %v", err)
		}
	}
}

func readMsgIdx(t *testing.T, path string) map[string]*statusEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var m map[string]*statusEntry
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return m
}

// ---- channel_show migration -------------------------------------------------

func TestMigrateChannelShow_LegacyMoves(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "channel_show.json")
	newPath := filepath.Join(dir, "configs", "channel_show.json")

	writeJSON(t, legacyPath, map[string]any{"mode": "all"})

	if err := Run(dir, dir, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fileExists(legacyPath) {
		t.Error("legacy channel_show.json still exists after migration")
	}
	if !fileExists(newPath) {
		t.Error("new channel_show.json not created")
	}
}

func TestMigrateChannelShow_NoOpWhenOnlyNewExists(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "configs", "channel_show.json")

	writeJSON(t, newPath, map[string]any{"mode": "all"})

	if err := Run(dir, dir, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !fileExists(newPath) {
		t.Error("new channel_show.json should still exist")
	}
}

func TestMigrateChannelShow_BothExistSkips(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "channel_show.json")
	newPath := filepath.Join(dir, "configs", "channel_show.json")

	writeJSON(t, legacyPath, map[string]any{"mode": "allowlist"})
	writeJSON(t, newPath, map[string]any{"mode": "all"})

	if err := Run(dir, dir, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both should still exist: legacy is NOT removed when new already exists.
	if !fileExists(legacyPath) {
		t.Error("legacy channel_show.json should be preserved when new also exists")
	}
	if !fileExists(newPath) {
		t.Error("new channel_show.json should be preserved")
	}
}

// ---- DM file migration ------------------------------------------------------

func TestMigrateDMFiles_LegacyRenames(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyFile := filepath.Join(chatPath, "DM_IK5FCK-10.jsonl")
	writeJSONL(t, legacyFile, []jsonRecord{{"msg_id": "a1", "msg": "hello"}})

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	newFile := filepath.Join(chatPath, "DM_IU5PMP_IK5FCK-10.jsonl")
	if !fileExists(newFile) {
		t.Errorf("expected new file %s to exist", newFile)
	}
	if fileExists(legacyFile) {
		t.Errorf("legacy file %s should have been removed", legacyFile)
	}
}

func TestMigrateDMFiles_AlreadyMigratedUnchanged(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	migratedFile := filepath.Join(chatPath, "DM_IU5PMP_IK5FCK-10.jsonl")
	writeJSONL(t, migratedFile, []jsonRecord{{"msg_id": "a1", "msg": "already here"}})

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !fileExists(migratedFile) {
		t.Error("already-migrated file should be untouched")
	}
	recs, err := readJSONLines(migratedFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Errorf("expected 1 record, got %d", len(recs))
	}
}

func TestMigrateDMFiles_MergeOnCollision(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyFile := filepath.Join(chatPath, "DM_IK5FCK-10.jsonl")
	newFile := filepath.Join(chatPath, "DM_IU5PMP_IK5FCK-10.jsonl")

	writeJSONL(t, legacyFile, []jsonRecord{
		{"msg_id": "a1", "msg": "from legacy"},
		{"msg_id": "shared", "msg": "dup"},
	})
	writeJSONL(t, newFile, []jsonRecord{
		{"msg_id": "b1", "msg": "already migrated"},
		{"msg_id": "shared", "msg": "dup"},
	})

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fileExists(legacyFile) {
		t.Error("legacy file should be removed after merge")
	}
	recs, err := readJSONLines(newFile)
	if err != nil {
		t.Fatal(err)
	}
	// b1 + shared + a1 = 3 unique records (shared deduplicated)
	if len(recs) != 3 {
		t.Errorf("expected 3 merged records, got %d", len(recs))
	}
}

func TestMigrateDMFiles_PreservesPublicFiles(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	broadcastFile := filepath.Join(chatPath, "P_broadcast.jsonl")
	channelFile := filepath.Join(chatPath, "P_123.jsonl")
	writeJSONL(t, broadcastFile, []jsonRecord{{"msg": "broadcast"}})
	writeJSONL(t, channelFile, []jsonRecord{{"msg": "channel"}})

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !fileExists(broadcastFile) {
		t.Error("P_broadcast.jsonl must be preserved")
	}
	if !fileExists(channelFile) {
		t.Error("P_123.jsonl must be preserved")
	}
}

// ---- msg_idx migration ------------------------------------------------------

func TestMigrateMsgIdx_LegacyKeyMigrates(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	idx := map[string]*statusEntry{
		"DM_IK5FCK-10": {LastMsgReceived: ts, UnreadCount: 2, LastMsg: "hi"},
		"P_broadcast":  {LastMsgReceived: ts, UnreadCount: 1},
	}
	writeJSON(t, filepath.Join(chatPath, "msg_idx.json"), idx)

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readMsgIdx(t, filepath.Join(chatPath, "msg_idx.json"))

	if _, ok := got["DM_IK5FCK-10"]; ok {
		t.Error("legacy key DM_IK5FCK-10 should have been removed")
	}
	if e, ok := got["DM_IU5PMP-1_IK5FCK-10"]; !ok {
		t.Error("expected new key DM_IU5PMP-1_IK5FCK-10")
	} else if e.UnreadCount != 2 {
		t.Errorf("UnreadCount = %d, want 2", e.UnreadCount)
	}
	if _, ok := got["P_broadcast"]; !ok {
		t.Error("P_broadcast key must be preserved")
	}
}

func TestMigrateMsgIdx_AlreadyMigratedUnchanged(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	idx := map[string]*statusEntry{
		"DM_IU5PMP-1_IK5FCK-10": {LastMsgReceived: ts, UnreadCount: 3},
	}
	writeJSON(t, filepath.Join(chatPath, "msg_idx.json"), idx)

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readMsgIdx(t, filepath.Join(chatPath, "msg_idx.json"))
	if e, ok := got["DM_IU5PMP-1_IK5FCK-10"]; !ok {
		t.Error("already-migrated key must be preserved")
	} else if e.UnreadCount != 3 {
		t.Errorf("UnreadCount = %d, want 3", e.UnreadCount)
	}
}

func TestMigrateMsgIdx_CollisionKeepsMostRecent(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	older := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	idx := map[string]*statusEntry{
		"DM_IK5FCK-10":          {LastMsgReceived: older, LastMsg: "legacy"},
		"DM_IU5PMP-1_IK5FCK-10": {LastMsgReceived: newer, LastMsg: "migrated"},
	}
	writeJSON(t, filepath.Join(chatPath, "msg_idx.json"), idx)

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readMsgIdx(t, filepath.Join(chatPath, "msg_idx.json"))
	if e, ok := got["DM_IU5PMP-1_IK5FCK-10"]; !ok {
		t.Error("key DM_IU5PMP-1_IK5FCK-10 must exist")
	} else if e.LastMsg != "migrated" {
		t.Errorf("LastMsg = %q, want %q (most recent entry)", e.LastMsg, "migrated")
	}
}

func TestMigrateMsgIdx_NoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("Run on missing msg_idx.json: %v", err)
	}
}

// ---- empty myCall -----------------------------------------------------------

func TestRun_EmptyMyCallSkipsDMAndIdx(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Legacy DM file should NOT be renamed when myCall is empty.
	legacyFile := filepath.Join(chatPath, "DM_IK5FCK-10.jsonl")
	writeJSONL(t, legacyFile, []jsonRecord{{"msg_id": "x", "msg": "hi"}})

	if err := Run(dir, chatPath, ""); err != nil {
		t.Fatalf("Run with empty myCall: %v", err)
	}

	if !fileExists(legacyFile) {
		t.Error("legacy DM file must not be renamed when myCall is empty")
	}
}

// ---- idempotency ------------------------------------------------------------

func TestRun_Idempotent(t *testing.T) {
	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat")
	if err := os.MkdirAll(chatPath, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyFile := filepath.Join(chatPath, "DM_IK5FCK-10.jsonl")
	writeJSONL(t, legacyFile, []jsonRecord{{"msg_id": "a1", "msg": "hello"}})

	ts := time.Now().UTC()
	idx := map[string]*statusEntry{
		"DM_IK5FCK-10": {LastMsgReceived: ts, UnreadCount: 1},
	}
	writeJSON(t, filepath.Join(chatPath, "msg_idx.json"), idx)

	// First run: migrate.
	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	// Second run: must be a no-op.
	if err := Run(dir, chatPath, "IU5PMP-1"); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	got := readMsgIdx(t, filepath.Join(chatPath, "msg_idx.json"))
	if _, ok := got["DM_IU5PMP-1_IK5FCK-10"]; !ok {
		t.Error("key DM_IU5PMP-1_IK5FCK-10 must exist after idempotent runs")
	}
}
