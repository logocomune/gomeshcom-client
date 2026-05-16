package positions

import (
	"encoding/json"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestPropertyRecordSerialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		record := Record{
			Latitude:   rapid.Float64Range(-90, 90).Draw(t, "Lat"),
			Longitude:  rapid.Float64Range(-180, 180).Draw(t, "Lng"),
			Altitude:   rapid.Int().Draw(t, "Alt"),
			HardwareID: rapid.String().Draw(t, "HwID"),
			FirstSeen:  time.Unix(rapid.Int64Range(0, 253402300799).Draw(t, "FirstSeen"), 0).UTC(),
			LastSeen:   time.Unix(rapid.Int64Range(0, 253402300799).Draw(t, "LastSeen"), 0).UTC(),
			RSSI:       rapid.Int().Draw(t, "RSSI"),
			SNR:        rapid.Int().Draw(t, "SNR"),
			Via:        rapid.SliceOf(rapid.String()).Draw(t, "Via"),
		}

		data, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var decoded Record
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if record.Latitude != decoded.Latitude {
			t.Errorf("Latitude mismatch: %v != %v", record.Latitude, decoded.Latitude)
		}
		if !record.FirstSeen.Equal(decoded.FirstSeen) {
			t.Errorf("FirstSeen mismatch: %v != %v", record.FirstSeen, decoded.FirstSeen)
		}
	})
}
