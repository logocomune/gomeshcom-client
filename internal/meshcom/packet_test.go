package meshcom

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePacket(t *testing.T) {
	tests := map[string]struct {
		data     string
		wantType PacketType
		wantErr  bool
	}{
		"text message": {
			data:     `{"src_type":"node","type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello","msg_id":"ABC","firmware":"4.35","fw_sub":"p","rssi":-90,"snr":8}`,
			wantType: PacketTypeMessage,
		},
		"position": {
			data:     `{"src_type":"node","type":"pos","src":"QQ1ABC-1","msg":"","lat":48,"lat_dir":"N","long":16,"long_dir":"E","aprs_symbol":"#","aprs_symbol_group":"/","hw_id":"MAC","msg_id":"ABC","alt":123,"batt":85,"firmware":"4.35","fw_sub":"p","rssi":-90,"snr":8}`,
			wantType: PacketTypePosition,
		},
		"position with numeric hardware fields": {
			data:     `{"src_type":"lora","type":"pos","src":"QQ5EKX-11,QQ5AKT-10,QQ5PFI-1","msg":"","lat":43.5076,"lat_dir":"N","long":10.3476,"long_dir":"E","aprs_symbol":"&","aprs_symbol_group":"/","hw_id":4,"msg_id":"AB39600F","alt":367,"batt":0,"firmware":35,"fw_sub":"p","rssi":-108,"snr":1}`,
			wantType: PacketTypePosition,
		},
		"telemetry": {
			data:     `{"src_type":"node","type":"tele","src":"QQ1ABC-1","temp1":9.99,"temp2":8.88,"hum":50,"qfe":999.9,"qnh":1001.1,"gas":9.9,"co2":400}`,
			wantType: PacketTypeTelemetry,
		},
		"telemetry with battery": {
			data:     `{"src_type":"lora","type":"tele","src":"QQ5EKX-11,QQ5AKT-10,QQ5PFI-1","batt":0,"temp1":0,"temp2":0,"hum":0,"qfe":0,"qnh":0,"gas":0,"co2":0}`,
			wantType: PacketTypeTelemetry,
		},
		"unknown field retained": {
			data:     `{"type":"msg","dst":"*","msg":"hello","future":"value"}`,
			wantType: PacketTypeMessage,
		},
		"numeric firmware tolerated": {
			data:     `{"src_type":"lora","type":"msg","src":"QQ1XAR-32,QQ5PFI-12","dst":"*","msg":"{CET}2026-05-14 19:14:19","msg_id":"6A01DD09","firmware":0,"fw_sub":"#","rssi":-108,"snr":0}`,
			wantType: PacketTypeMessage,
		},
		"invalid json": {
			data:    `{`,
			wantErr: true,
		},
		"missing type": {
			data:    `{"msg":"hello"}`,
			wantErr: true,
		},
		"unsupported type": {
			data:    `{"type":"unknown","msg":"hello"}`,
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			packet, err := ParsePacket([]byte(test.data))
			if (err != nil) != test.wantErr {
				t.Fatalf("ParsePacket() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if packet.Type != test.wantType {
				t.Fatalf("ParsePacket() type = %q, want %q", packet.Type, test.wantType)
			}
			if strings.Contains(test.data, `"future"`) {
				if _, ok := packet.Unknown["future"]; !ok {
					t.Fatal("unknown field not retained")
				}
			}
		})
	}
}

func TestParsePacket_TypePreservedInJSON(t *testing.T) {
	inputs := map[string]PacketType{
		`{"src_type":"lora","type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello","rssi":-90,"snr":8}`:    PacketTypeMessage,
		`{"src_type":"lora","type":"pos","src":"QQ1ABC-1","lat":48.1,"long":16.3,"alt":200,"batt":80}`:    PacketTypePosition,
		`{"src_type":"lora","type":"tele","src":"QQ1ABC-1","temp1":20.0,"hum":55,"batt":90,"qnh":1013.0}`: PacketTypeTelemetry,
	}

	for input, wantType := range inputs {
		t.Run(string(wantType), func(t *testing.T) {
			envelope, err := ParsePacket([]byte(input))
			if err != nil {
				t.Fatalf("ParsePacket() error = %v", err)
			}

			out, err := json.Marshal(envelope.Packet)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var fields map[string]json.RawMessage
			if err := json.Unmarshal(out, &fields); err != nil {
				t.Fatalf("unmarshal output: %v", err)
			}

			raw, ok := fields["type"]
			if !ok {
				t.Fatal("marshaled packet missing \"type\" field")
			}

			var got PacketType
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode type field: %v", err)
			}

			if got != wantType {
				t.Fatalf("type = %q, want %q", got, wantType)
			}
		})
	}
}

func TestNewOutgoingText(t *testing.T) {
	tests := map[string]struct {
		destination string
		message     string
		maxLength   int
		wantErr     bool
	}{
		"broadcast": {
			destination: "*",
			message:     "hello",
			maxLength:   149,
		},
		"callsign": {
			destination: "QQ1ABC-1",
			message:     "hello",
			maxLength:   149,
		},
		"group": {
			destination: "10",
			message:     "hello",
			maxLength:   149,
		},
		"missing destination": {
			message:   "hello",
			maxLength: 149,
			wantErr:   true,
		},
		"missing message": {
			destination: "*",
			maxLength:   149,
			wantErr:     true,
		},
		"too long": {
			destination: "*",
			message:     strings.Repeat("a", 150),
			maxLength:   149,
			wantErr:     true,
		},
		"unicode length": {
			destination: "*",
			message:     strings.Repeat("å", 149),
			maxLength:   149,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, err := NewOutgoingText(test.destination, test.message, test.maxLength)
			if (err != nil) != test.wantErr {
				t.Fatalf("NewOutgoingText() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if message.Type != PacketTypeMessage {
				t.Fatalf("NewOutgoingText() type = %q, want %q", message.Type, PacketTypeMessage)
			}
		})
	}
}
