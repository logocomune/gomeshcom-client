package chatlog

import (
	"encoding/json"
	"testing"
	"time"

	"pgregory.net/rapid"
)

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
