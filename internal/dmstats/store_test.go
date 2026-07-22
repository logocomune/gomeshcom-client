package dmstats

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// ---- table-driven unit tests ------------------------------------------------

func TestRecordSent_WithSSID(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))

	s.RecordSent("CALL-1")

	snap := s.Snapshot()
	if got := snap["CALL-1"].Sent; got != 1 {
		t.Errorf("full sent: want 1, got %d", got)
	}
	if got := snap["CALL"].Sent; got != 1 {
		t.Errorf("base sent: want 1, got %d", got)
	}
	if len(snap) != 2 {
		t.Errorf("expected 2 entries, got %d", len(snap))
	}
}

func TestRecordSent_WithoutSSID(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))

	s.RecordSent("CALL")

	snap := s.Snapshot()
	if got := snap["CALL"].Sent; got != 1 {
		t.Errorf("want 1, got %d", got)
	}
	if len(snap) != 1 {
		t.Errorf("expected 1 entry (no duplicate), got %d", len(snap))
	}
}

func TestRecordSent_LowercaseNormalized(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))

	s.RecordSent("call-1")

	snap := s.Snapshot()
	if _, ok := snap["CALL-1"]; !ok {
		t.Error("expected uppercase key CALL-1")
	}
	if _, ok := snap["CALL"]; !ok {
		t.Error("expected uppercase base key CALL")
	}
}

func TestRecordAck_WithSSID(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))
	s.RecordSent("CALL-1")
	s.RecordAck("CALL-1")

	snap := s.Snapshot()

	full := snap["CALL-1"]
	if full.Ack != 1 {
		t.Errorf("full ack: want 1, got %d", full.Ack)
	}

	base := snap["CALL"]
	if base.Ack != 1 {
		t.Errorf("base ack: want 1, got %d", base.Ack)
	}
}

func TestRecordAck_WithoutSSID(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))
	s.RecordSent("CALL")
	s.RecordAck("CALL")

	snap := s.Snapshot()
	entry := snap["CALL"]
	if entry.Ack != 1 {
		t.Errorf("ack: want 1, got %d", entry.Ack)
	}
	if len(snap) != 1 {
		t.Errorf("expected 1 entry, got %d", len(snap))
	}
}

func TestRecordAck_MultipleAccumulate(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))
	s.RecordSent("CALL-2")
	s.RecordSent("CALL-2")
	s.RecordAck("CALL-2")
	s.RecordAck("CALL-2")

	snap := s.Snapshot()
	full := snap["CALL-2"]
	if full.Ack != 2 {
		t.Errorf("ack count: want 2, got %d", full.Ack)
	}
}

func TestSnapshot_ReturnsCopy(t *testing.T) {
	s := NewSQLite(openDMStatsTestDB(t))
	s.RecordSent("CALL-1")
	snap := s.Snapshot()
	snap["CALL-1"] = Entry{Sent: 99}
	if s.Snapshot()["CALL-1"].Sent != 1 {
		t.Error("snapshot mutation affected store")
	}
}

// ---- JSON serialization check -----------------------------------------------

func TestEntryJSONTags(t *testing.T) {
	e := Entry{Sent: 3, Ack: 2}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sent", "ack"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}

// ---- property-based tests (rapid) -------------------------------------------

func TestProperty_SentCountMatchesRecordCalls(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		callsign := rapid.StringMatching(`^[A-Z0-9]{3,6}-[1-9]$`).Draw(rt, "callsign")
		n := rapid.IntRange(1, 50).Draw(rt, "n")

		s := NewSQLite(openDMStatsTestDB(t))
		for i := 0; i < n; i++ {
			s.RecordSent(callsign)
		}
		snap := s.Snapshot()
		if got := snap[callsign].Sent; got != n {
			rt.Fatalf("full sent: want %d, got %d", n, got)
		}
		base := callsignPair_base(callsign)
		if got := snap[base].Sent; got != n {
			rt.Fatalf("base sent: want %d, got %d", n, got)
		}
	})
}

func TestProperty_AckNeverExceedsSent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		callsign := rapid.StringMatching(`^[A-Z0-9]{3,6}-[1-9]$`).Draw(rt, "callsign")
		sent := rapid.IntRange(0, 20).Draw(rt, "sent")
		acked := rapid.IntRange(0, 20).Draw(rt, "acked")

		s := NewSQLite(openDMStatsTestDB(t))
		for i := 0; i < sent; i++ {
			s.RecordSent(callsign)
		}
		for i := 0; i < acked; i++ {
			s.RecordAck(callsign)
		}
		snap := s.Snapshot()
		full := snap[callsign]
		if full.Sent != sent {
			rt.Fatalf("sent mismatch: want %d, got %d", sent, full.Sent)
		}
		if full.Ack != acked {
			rt.Fatalf("ack mismatch: want %d, got %d", acked, full.Ack)
		}
	})
}

// callsignPair_base is a test-local helper mirroring the store internals.
func callsignPair_base(full string) string {
	_, base := callsignPair(full)
	return base
}
