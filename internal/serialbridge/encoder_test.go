package serialbridge

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

type encoderIdentity string

func (identity encoderIdentity) Current() string {
	return string(identity)
}

func TestEncoderFormatsCommands(t *testing.T) {
	tests := []struct {
		name        string
		identity    Identity
		destination string
		message     string
		want        string
	}{
		{
			name:        "broadcast",
			destination: "*",
			message:     "Hello everyone",
			want:        "::Hello everyone\r\n",
		},
		{
			name:        "channel",
			destination: "2321",
			message:     "Hello channel",
			want:        "::{2321}Hello channel\r\n",
		},
		{
			name:        "channel normalized",
			destination: "00001",
			message:     "Hello channel",
			want:        "::{1}Hello channel\r\n",
		},
		{
			name:        "direct message uppercased",
			identity:    encoderIdentity("QQ1OWN-1"),
			destination: " qq1peer-2 ",
			message:     "Hello direct",
			want:        "::{QQ1PEER-2}Hello direct\r\n",
		},
		{
			name:        "ack-like suffix remains unchanged",
			destination: "QQ1PEER-2",
			message:     "test{659",
			want:        "::{QQ1PEER-2}test{659\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewEncoder(tt.identity)
			got, err := encoder.Encode(TextCommand{
				Destination:     tt.destination,
				Message:         tt.message,
				MaxMessageRunes: 200,
			})
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Encode() = %q, want %q", got, tt.want)
			}
			if strings.Count(string(got), "\r\n") != 1 || !strings.HasSuffix(string(got), "\r\n") {
				t.Fatalf("command terminator = %q, want exactly one trailing CRLF", got)
			}
		})
	}
}

func TestEncoderRejectsInvalidCommands(t *testing.T) {
	tests := []struct {
		name        string
		identity    Identity
		destination string
		message     string
		maxRunes    int
		wantError   error
	}{
		{name: "empty destination", destination: "", message: "hello", maxRunes: 200},
		{name: "empty message", destination: "*", message: "", maxRunes: 200},
		{name: "application rune limit", destination: "*", message: "hello", maxRunes: 4},
		{name: "invalid utf8", destination: "*", message: string([]byte{0xff}), maxRunes: 200},
		{name: "zero channel", destination: "0", message: "hello", maxRunes: 200, wantError: ErrInvalidDestination},
		{name: "channel too large", destination: "100000", message: "hello", maxRunes: 200, wantError: ErrInvalidDestination},
		{name: "invalid callsign", destination: "QQ!", message: "hello", maxRunes: 200, wantError: ErrInvalidDestination},
		{
			name:        "self direct message",
			identity:    encoderIdentity("QQ1OWN-1"),
			destination: "qq1own-1",
			message:     "hello",
			maxRunes:    200,
			wantError:   ErrSelfDirectMessage,
		},
		{name: "line feed injection", destination: "*", message: "one\ntwo", maxRunes: 200, wantError: ErrCommandInjection},
		{name: "carriage return injection", destination: "*", message: "one\rtwo", maxRunes: 200, wantError: ErrCommandInjection},
		{name: "nul injection", destination: "*", message: "one\x00two", maxRunes: 200, wantError: ErrCommandInjection},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewEncoder(tt.identity)
			_, err := encoder.Encode(TextCommand{
				Destination:     tt.destination,
				Message:         tt.message,
				MaxMessageRunes: tt.maxRunes,
			})
			if err == nil {
				t.Fatal("Encode() error = nil")
			}
			if tt.wantError != nil && !errors.Is(err, tt.wantError) {
				t.Fatalf("Encode() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestEncoderEnforcesFirmwareByteLimit(t *testing.T) {
	tests := []struct {
		name        string
		destination string
		message     string
		wantBytes   int
		wantError   bool
	}{
		{
			name:        "broadcast exactly 160 bytes",
			destination: "*",
			message:     strings.Repeat("a", 160),
			wantBytes:   164,
		},
		{
			name:        "broadcast 161 bytes",
			destination: "*",
			message:     strings.Repeat("a", 161),
			wantError:   true,
		},
		{
			name:        "direct exactly 160 byte payload",
			destination: "QQ1ABC-1",
			message:     strings.Repeat("a", 150),
			wantBytes:   164,
		},
		{
			name:        "direct 161 byte payload",
			destination: "QQ1ABC-1",
			message:     strings.Repeat("a", 151),
			wantError:   true,
		},
		{
			name:        "multibyte utf8 counted as bytes",
			destination: "*",
			message:     strings.Repeat("è", 81),
			wantError:   true,
		},
	}

	encoder := NewEncoder(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encoder.Encode(TextCommand{
				Destination:     tt.destination,
				Message:         tt.message,
				MaxMessageRunes: 200,
			})
			if tt.wantError {
				if !errors.Is(err, ErrPayloadTooLong) {
					t.Fatalf("Encode() error = %v, want ErrPayloadTooLong", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if len(got) != tt.wantBytes {
				t.Fatalf("wire bytes = %d, want %d", len(got), tt.wantBytes)
			}
		})
	}
}

func FuzzEncoder(f *testing.F) {
	seeds := []struct {
		destination string
		message     string
	}{
		{destination: "*", message: "hello"},
		{destination: "2321", message: "channel"},
		{destination: "QQ1PEER-2", message: "direct"},
		{destination: "QQ1PEER-2", message: "test{659"},
		{destination: "QQ1PEER-2", message: "line\nbreak"},
		{destination: "0", message: "invalid channel"},
	}
	for _, seed := range seeds {
		f.Add(seed.destination, seed.message)
	}

	f.Fuzz(func(t *testing.T, destination, message string) {
		encoder := NewEncoder(encoderIdentity("QQ1OWN-1"))
		command, err := encoder.Encode(TextCommand{
			Destination:     destination,
			Message:         message,
			MaxMessageRunes: 512,
		})
		if err != nil {
			return
		}
		if !utf8.Valid(command) {
			t.Fatalf("encoded command is invalid UTF-8: %x", command)
		}
		if !strings.HasPrefix(string(command), "::") || !strings.HasSuffix(string(command), "\r\n") {
			t.Fatalf("invalid command framing: %q", command)
		}
		payload := command[2 : len(command)-2]
		if len(payload) > MaxFirmwarePayloadBytes {
			t.Fatalf("payload bytes = %d, limit %d", len(payload), MaxFirmwarePayloadBytes)
		}
		if bytesBeforeTerminator := command[:len(command)-2]; strings.ContainsAny(string(bytesBeforeTerminator), "\r\n\x00") {
			t.Fatalf("command injection bytes present: %q", command)
		}
	})
}
