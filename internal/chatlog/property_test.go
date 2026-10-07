package chatlog

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"

	"pgregory.net/rapid"
)

func TestPropertyConversationMetadataExcludesStoreNotices(t *testing.T) {
	db := openChatLogTestDB(t)
	logger := NewSQLite(db, staticCallsign("QQ0QQ-1"))
	base := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		if _, err := db.Exec("DELETE FROM chats_public"); err != nil {
			t.Fatal(err)
		}
		messages := rapid.SliceOfN(rapid.SampledFrom([]string{
			"hello", "QQ0QQ-1  :sto156 QQ1ABC-1", "QQ0QQ-1  :sto000",
			"QQ0QQ-1  :stoabc", "QQ0QQ-1  :sto156{156", "QQ0QQ-1  :sto156 :ACK571",
			"QQ0QQ-1  :sto156 :REJ571", "é12345678:sto156", "é1234567:sto156",
			"ack571", "rej571",
		}), 0, 30).Draw(t, "messages")
		var count int64
		var lastSeen time.Time
		for _, message := range messages {
			at := base.Add(time.Duration(rapid.IntRange(0, 100).Draw(t, "minutes")) * time.Minute)
			if err := logger.Append(meshcom.TextMessage{Source: "QQ1ABC-1", Destination: "*", Message: message}, at); err != nil {
				t.Fatal(err)
			}
			if !meshcom.IsStoreNotice(message) {
				count++
				if at.After(lastSeen) {
					lastSeen = at
				}
			}
		}
		conversations, err := logger.List()
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			if len(conversations) != 0 {
				t.Fatalf("notice-only conversations = %+v", conversations)
			}
		} else if len(conversations) != 1 || conversations[0].Size != count || !conversations[0].LastSeen.Equal(lastSeen) {
			t.Fatalf("conversations = %+v, want count %d at %v", conversations, count, lastSeen)
		}
		var stored int
		if err := db.QueryRow("SELECT COUNT(*) FROM chats_public").Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != len(messages) {
			t.Fatalf("stored rows = %d, want %d", stored, len(messages))
		}
	})
}

func TestPropertyRecordSerialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		record := Record{
			ReceivedAt: time.Unix(rapid.Int64Range(0, 253402300799).Draw(t, "ReceivedAt"), 0).UTC(),
			Src:        rapid.String().Draw(t, "Src"),
			SrcType:    rapid.String().Draw(t, "SrcType"),
			Dst:        rapid.String().Draw(t, "Dst"),
			MsgID:      rapid.String().Draw(t, "MsgID"),
			Msg:        rapid.String().Draw(t, "Msg"),
			RSSI:       rapid.Int().Draw(t, "RSSI"),
			SNR:        rapid.Int().Draw(t, "SNR"),
		}

		data, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var decoded Record
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		// JSON unmarshal for time might lose some precision, but here we use Unix seconds
		if !record.ReceivedAt.Equal(decoded.ReceivedAt) {
			t.Errorf("ReceivedAt mismatch: %v != %v", record.ReceivedAt, decoded.ReceivedAt)
		}
		if record.Src != decoded.Src {
			t.Errorf("Src mismatch: %q != %q", record.Src, decoded.Src)
		}
		if record.Msg != decoded.Msg {
			t.Errorf("Msg mismatch: %q != %q", record.Msg, decoded.Msg)
		}
		if record.RSSI != decoded.RSSI {
			t.Errorf("RSSI mismatch: %d != %d", record.RSSI, decoded.RSSI)
		}
	})
}
