package chatlog

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	_ "modernc.org/sqlite"
)

func testIntPtr(value int) *int { return &value }

type staticCallsign string

func (s staticCallsign) Current() string { return string(s) }

func TestSQLiteAppendReadListRemove(t *testing.T) {
	db := openChatLogTestDB(t)
	logger := NewSQLite(db, staticCallsign("QQ0QQ-1"))
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	if err := logger.Append(meshcom.TextMessage{Source: "SRC-1", Destination: "*", MessageID: "PUB1", Message: "broadcast", RSSI: testIntPtr(-70), SNR: testIntPtr(8)}, base); err != nil {
		t.Fatalf("Append public error = %v", err)
	}
	if err := logger.Append(meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "QQ0QQ-1", MessageID: "DM1", Message: "dm"}, base.Add(time.Minute)); err != nil {
		t.Fatalf("Append dm error = %v", err)
	}

	publicRecords, err := logger.ReadSince("P_broadcast", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince public error = %v", err)
	}
	if len(publicRecords) != 1 || publicRecords[0].Msg != "broadcast" || publicRecords[0].RSSI != -70 || publicRecords[0].SNR != 8 {
		t.Fatalf("public records = %+v", publicRecords)
	}

	dmRecords, err := logger.ReadSince("DM_QQ0QQ_QQ1ABC-1", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince dm error = %v", err)
	}
	if len(dmRecords) != 1 || dmRecords[0].Msg != "dm" {
		t.Fatalf("dm records = %+v", dmRecords)
	}

	convs, err := logger.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("conversation count = %d, want 2", len(convs))
	}

	contains, err := logger.FileContainsSSID("DM_QQ0QQ_QQ1ABC-1", "QQ0QQ-1")
	if err != nil {
		t.Fatalf("FileContainsSSID() error = %v", err)
	}
	if !contains {
		t.Fatal("FileContainsSSID() = false, want true")
	}

	if err := logger.Remove("P_broadcast"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	publicRecords, err = logger.ReadSince("P_broadcast", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince after remove error = %v", err)
	}
	if len(publicRecords) != 0 {
		t.Fatalf("public records after remove = %d, want 0", len(publicRecords))
	}
}

func TestSQLiteReadSinceDeduplicates(t *testing.T) {
	db := openChatLogTestDB(t)
	logger := NewSQLite(db, staticCallsign("QQ0QQ-1"))
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	msg := meshcom.TextMessage{Source: "SRC-1", Destination: "123", MessageID: "DUP", Message: "first"}
	if err := logger.Append(msg, base); err != nil {
		t.Fatalf("append first: %v", err)
	}
	msg.Message = "duplicate"
	if err := logger.Append(msg, base.Add(time.Minute)); err != nil {
		t.Fatalf("append duplicate: %v", err)
	}
	msg.Message = "later"
	if err := logger.Append(msg, base.Add(10*time.Minute)); err != nil {
		t.Fatalf("append later: %v", err)
	}

	records, err := logger.ReadSince("P_123", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 2 || records[0].Msg != "first" || records[1].Msg != "later" {
		t.Fatalf("records = %+v", records)
	}
}

func TestSQLiteAppendStoresSourceOriginAndVia(t *testing.T) {
	db := openChatLogTestDB(t)
	logger := NewSQLite(db, staticCallsign("QQ0QQ-1"))
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	if err := logger.Append(meshcom.TextMessage{Source: "SRC-1,RELAY-1,RELAY-2", Destination: "*", MessageID: "PUBVIA", Message: "broadcast"}, base); err != nil {
		t.Fatalf("Append public error = %v", err)
	}
	if err := logger.Append(meshcom.TextMessage{Source: "QQ1ABC-1,RELAY-3", Destination: "QQ0QQ-1", MessageID: "DMVIA", Message: "dm"}, base.Add(time.Minute)); err != nil {
		t.Fatalf("Append dm error = %v", err)
	}

	publicRecords, err := logger.ReadSince("P_broadcast", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince public error = %v", err)
	}
	if publicRecords[0].Src != "SRC-1" {
		t.Fatalf("public Src = %q, want SRC-1", publicRecords[0].Src)
	}
	if len(publicRecords[0].Via) != 2 || publicRecords[0].Via[0] != "RELAY-1" || publicRecords[0].Via[1] != "RELAY-2" {
		t.Fatalf("public Via = %+v, want [RELAY-1 RELAY-2]", publicRecords[0].Via)
	}

	dmRecords, err := logger.ReadSince("DM_QQ0QQ_QQ1ABC-1", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince dm error = %v", err)
	}
	if dmRecords[0].Src != "QQ1ABC-1" {
		t.Fatalf("dm Src = %q, want QQ1ABC-1", dmRecords[0].Src)
	}
	if len(dmRecords[0].Via) != 1 || dmRecords[0].Via[0] != "RELAY-3" {
		t.Fatalf("dm Via = %+v, want [RELAY-3]", dmRecords[0].Via)
	}
}

func TestSQLiteAppendRecordsAckOnRecentOutboundSequence(t *testing.T) {
	db := openChatLogTestDB(t)
	logger := NewSQLite(db, staticCallsign("QQ0QQ-1"))
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	oldRecord, err := logger.AppendFailed("QQ0QQ-1", "QQ1ABC-1", "old {1234", base.Add(-6*time.Minute))
	if err != nil {
		t.Fatalf("append old outbound: %v", err)
	}
	recentRecord, err := logger.AppendFailed("QQ0QQ-1", "QQ1ABC-1", "recent {1234", base.Add(-4*time.Minute))
	if err != nil {
		t.Fatalf("append recent outbound: %v", err)
	}
	if oldRecord.SequenceID != "1234" || recentRecord.SequenceID != "1234" {
		t.Fatalf("sequence ids = %q/%q, want 1234/1234", oldRecord.SequenceID, recentRecord.SequenceID)
	}

	ackRSSI := -81
	ackSNR := 6
	err = logger.Append(meshcom.TextMessage{
		Source:      "QQ1ABC-1,RELAY-1",
		SourceType:  "lora",
		Destination: "QQ0QQ-1",
		MessageID:   "ACK1",
		Message:     ":1234",
		RSSI:        &ackRSSI,
		SNR:         &ackSNR,
	}, base)
	if err != nil {
		t.Fatalf("append ack: %v", err)
	}

	records, err := logger.ReadSince("DM_QQ0QQ_QQ1ABC-1", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("len(records) = %d, want 3: %+v", len(records), records)
	}
	if records[0].AckStatus != "" {
		t.Fatalf("old AckStatus = %q, want empty", records[0].AckStatus)
	}
	if records[1].AckStatus != "ack" {
		t.Fatalf("recent AckStatus = %q, want ack", records[1].AckStatus)
	}
	if !records[1].AckReceivedAt.Equal(base) {
		t.Fatalf("AckReceivedAt = %s, want %s", records[1].AckReceivedAt, base)
	}
	if records[1].AckSrc != "QQ1ABC-1" || records[1].AckRSSI != ackRSSI || records[1].AckSNR != ackSNR {
		t.Fatalf("ack metadata = %+v", records[1])
	}
	if len(records[1].AckVia) != 1 || records[1].AckVia[0] != "RELAY-1" {
		t.Fatalf("AckVia = %+v, want [RELAY-1]", records[1].AckVia)
	}
}

func openChatLogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE chats_dm (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			msg_id TEXT,
			sequence_id TEXT,
			received_at TEXT NOT NULL,
			src TEXT,
			src_type TEXT,
			via TEXT,
			dst TEXT NOT NULL,
			msg TEXT NOT NULL,
			rssi INTEGER,
			snr INTEGER,
			direction TEXT,
			delivery_status TEXT,
			ack_status TEXT,
			ack_received_at TEXT,
			ack_src TEXT,
			ack_src_type TEXT,
			ack_rssi INTEGER,
			ack_snr INTEGER,
			ack_via TEXT
		)`,
		`CREATE TABLE chats_public (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			channel TEXT,
			msg_id TEXT,
			sequence_id TEXT,
			received_at TEXT NOT NULL,
			src TEXT,
			src_type TEXT,
			via TEXT,
			dst TEXT NOT NULL,
			msg TEXT NOT NULL,
			rssi INTEGER,
			snr INTEGER
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("create chat table: %v", err)
		}
	}
	return db
}
