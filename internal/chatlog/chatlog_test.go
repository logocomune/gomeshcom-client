package chatlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

func writeJSONL(t *testing.T, path string, records []Record) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode record: %v", err)
		}
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, "QQ0QQ-1")

	t0 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	rec := func(at time.Time) Record { return Record{ReceivedAt: at, Msg: "hi"} }

	writeJSONL(t, filepath.Join(dir, "P_broadcast.jsonl"), []Record{rec(t0)})
	writeJSONL(t, filepath.Join(dir, "P_123.jsonl"), []Record{rec(t1)})
	writeJSONL(t, filepath.Join(dir, "DM_QQ1ABC-1.jsonl"), []Record{rec(t2)})
	writeJSONL(t, filepath.Join(dir, "unknown.jsonl"), []Record{rec(t0)}) // should be skipped

	convs, err := logger.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(convs) != 3 {
		t.Fatalf("got %d conversations, want 3", len(convs))
	}

	byID := map[string]Conversation{}
	for _, c := range convs {
		byID[c.ID] = c
	}

	if c, ok := byID["P_broadcast"]; !ok || c.Kind != "broadcast" || c.Label != "Broadcast" {
		t.Errorf("P_broadcast = %+v", byID["P_broadcast"])
	}
	if c, ok := byID["P_123"]; !ok || c.Kind != "channel" || c.Label != "123" {
		t.Errorf("P_123 = %+v", byID["P_123"])
	}
	if c, ok := byID["DM_QQ1ABC-1"]; !ok || c.Kind != "dm" || c.Label != "QQ1ABC-1" {
		t.Errorf("DM_QQ1ABC-1 = %+v", byID["DM_QQ1ABC-1"])
	}
}

func TestListMissingDir(t *testing.T) {
	logger := New(filepath.Join(t.TempDir(), "nonexistent"), "QQ0QQ-1")
	convs, err := logger.List()
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(convs) != 0 {
		t.Fatalf("expected empty slice, got %d", len(convs))
	}
}

func TestReadSince(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, "QQ0QQ-1")

	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	records := []Record{
		{ReceivedAt: base, Msg: "old"},
		{ReceivedAt: base.Add(30 * time.Minute), Msg: "mid"},
		{ReceivedAt: base.Add(60 * time.Minute), Msg: "new"},
	}
	writeJSONL(t, filepath.Join(dir, "P_broadcast.jsonl"), records)

	got, err := logger.ReadSince("P_broadcast", base.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0].Msg != "mid" || got[1].Msg != "new" {
		t.Errorf("unexpected records: %+v", got)
	}
}

func TestReadSinceMissingFile(t *testing.T) {
	logger := New(t.TempDir(), "QQ0QQ-1")
	records, err := logger.ReadSince("P_broadcast", time.Now())
	if err != nil {
		t.Fatalf("ReadSince missing file: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty slice, got %d", len(records))
	}
}

func TestReadSinceEmptyNeverNilSlice(t *testing.T) {
	logger := New(t.TempDir(), "QQ0QQ-1")
	records, err := logger.ReadSince("P_broadcast", time.Now())
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	b, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) == "null" {
		t.Fatal("ReadSince returned nil slice: JSON would serialize as null, breaking frontend")
	}
}

func TestReadSinceInvalidID(t *testing.T) {
	logger := New(t.TempDir(), "QQ0QQ-1")
	cases := []string{
		"../etc/passwd",
		"../../secret",
		"P_",
		"DM_",
		"random",
		"P_abc",
	}
	for _, id := range cases {
		_, err := logger.ReadSince(id, time.Now())
		if err == nil {
			t.Errorf("ReadSince(%q) expected error, got nil", id)
		}
	}
}

func TestReadSinceMalformedLineSkipped(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, "QQ0QQ-1")

	path := filepath.Join(dir, "P_broadcast.jsonl")
	f, _ := os.Create(path)
	f.WriteString("not json\n")
	enc := json.NewEncoder(f)
	enc.Encode(Record{ReceivedAt: time.Now(), Msg: "good"})
	f.Close()

	records, err := logger.ReadSince("P_broadcast", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(records) != 1 || records[0].Msg != "good" {
		t.Errorf("unexpected records: %+v", records)
	}
}

func TestAppendDMFilter(t *testing.T) {
	myCall := "QQ0QQ-1"
	now := time.Now().UTC()

	msg := func(src, dst, text string) meshcom.TextMessage {
		return meshcom.TextMessage{Source: src, Destination: dst, Message: text}
	}

	cases := []struct {
		msg      meshcom.TextMessage
		wantLog  bool
		wantFile string // expected JSONL file (without .jsonl), empty = use filename(dst)
	}{
		{msg("QQ1ABC-1", "*", "broadcast"), true, "P_broadcast"},
		{msg("QQ1ABC-1", "123", "channel"), true, "P_123"},
		{msg(myCall, "QQ1ABC-1", "outgoing DM"), true, "DM_QQ1ABC-1"}, // interlocutor = dst
		{msg("QQ1ABC-1", myCall, "incoming DM"), true, "DM_QQ1ABC-1"}, // interlocutor = src
		{msg("QQ1ABC-1", "QQ1XYZ", "unrelated DM"), false, ""},
	}

	for _, tc := range cases {
		name := tc.msg.Source + "→" + tc.msg.Destination
		t.Run(name, func(t *testing.T) {
			logDir := t.TempDir()
			l := New(logDir, myCall)
			if err := l.Append(tc.msg, now); err != nil {
				t.Fatalf("Append: %v", err)
			}
			if !tc.wantLog {
				return
			}
			recs, err := l.ReadSince(tc.wantFile, time.Time{})
			if err != nil {
				t.Fatalf("ReadSince(%q): %v", tc.wantFile, err)
			}
			if len(recs) == 0 {
				t.Fatalf("expected record in %q, got none", tc.wantFile)
			}
		})
	}
}

func TestAppendFailedPersistsOutboundStatus(t *testing.T) {
	now := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		dst        string
		wantID     string
		wantStatus string
	}{
		"broadcast": {dst: "*", wantID: "P_broadcast", wantStatus: "failed"},
		"channel":   {dst: "123", wantID: "P_123", wantStatus: "failed"},
		"dm":        {dst: "QQ1ABC-1", wantID: "DM_QQ1ABC-1", wantStatus: "failed"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := New(t.TempDir(), "DIFFERENT-1")
			record, err := logger.AppendFailed("QQ0QQ-1", test.dst, "hello", now)
			if err != nil {
				t.Fatalf("AppendFailed: %v", err)
			}
			if record.Direction != "outbound" {
				t.Fatalf("Direction = %q, want outbound", record.Direction)
			}
			if record.DeliveryStatus != test.wantStatus {
				t.Fatalf("DeliveryStatus = %q, want %q", record.DeliveryStatus, test.wantStatus)
			}

			records, err := logger.ReadSince(test.wantID, time.Time{})
			if err != nil {
				t.Fatalf("ReadSince(%q): %v", test.wantID, err)
			}
			if len(records) != 1 {
				t.Fatalf("len(records) = %d, want 1", len(records))
			}
			if records[0].Src != "QQ0QQ-1" {
				t.Fatalf("Src = %q, want QQ0QQ-1", records[0].Src)
			}
			if records[0].DeliveryStatus != "failed" {
				t.Fatalf("DeliveryStatus = %q, want failed", records[0].DeliveryStatus)
			}
		})
	}
}

// TestAppendDMNormalization verifies that both directions of a DM conversation
// land in the same file keyed on the interlocutor's callsign.
func TestAppendDMNormalization(t *testing.T) {
	myCall := "QQ0QQ-1"
	other := "QQ1ABC-1"
	now := time.Now().UTC()
	logDir := t.TempDir()
	l := New(logDir, myCall)

	outgoing := meshcom.TextMessage{Source: myCall, Destination: other, Message: "hello"}
	incoming := meshcom.TextMessage{Source: other, Destination: myCall, Message: "world"}

	if err := l.Append(outgoing, now); err != nil {
		t.Fatalf("Append outgoing: %v", err)
	}
	if err := l.Append(incoming, now.Add(time.Second)); err != nil {
		t.Fatalf("Append incoming: %v", err)
	}

	// Both must appear in DM_QQ1ABC-1, not in DM_QQ0QQ-1.
	recs, err := l.ReadSince("DM_QQ1ABC-1", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records in DM_%s, got %d", other, len(recs))
	}

	// DM_QQ0QQ-1 must not exist.
	wrongRecs, _ := l.ReadSince("DM_QQ0QQ-1", time.Time{})
	if len(wrongRecs) > 0 {
		t.Fatalf("unexpected records in DM_%s: %d", myCall, len(wrongRecs))
	}
}

func TestRemoveDeletesFile(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, "QQ0QQ-1")

	msg := meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "*", Message: "hi"}
	if err := logger.Append(msg, time.Now()); err != nil {
		t.Fatalf("Append: %v", err)
	}

	convs, _ := logger.List()
	if len(convs) != 1 {
		t.Fatalf("expected 1 conversation before remove, got %d", len(convs))
	}

	if err := logger.Remove("P_broadcast"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "P_broadcast.jsonl")); !os.IsNotExist(err) {
		t.Fatal("file still exists after Remove")
	}

	convs, _ = logger.List()
	for _, c := range convs {
		if c.ID == "P_broadcast" {
			t.Fatal("P_broadcast still appears in List after Remove")
		}
	}
}

func TestRemoveIdempotentMissingFile(t *testing.T) {
	logger := New(t.TempDir(), "QQ0QQ-1")
	if err := logger.Remove("P_broadcast"); err != nil {
		t.Fatalf("Remove on missing file: %v", err)
	}
}

func TestRemoveInvalidID(t *testing.T) {
	logger := New(t.TempDir(), "QQ0QQ-1")
	cases := []string{"../etc/passwd", "../../secret", "random", "DM_", "P_"}
	for _, id := range cases {
		if err := logger.Remove(id); err == nil {
			t.Errorf("Remove(%q) expected error, got nil", id)
		}
	}
}

func TestRemoveConcurrentWithAppend(t *testing.T) {
	dir := t.TempDir()
	logger := New(dir, "QQ0QQ-1")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			msg := meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "*", Message: "x"}
			_ = logger.Append(msg, time.Now())
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = logger.Remove("P_broadcast")
		}
	}()

	wg.Wait()
}
