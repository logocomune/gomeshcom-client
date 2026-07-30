package serialbridge

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestDecoderExtractsVerifiedFirmwareCapture(t *testing.T) {
	capture, err := os.ReadFile("testdata/firmware-4.35.log")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	decoder, err := NewDecoder(1024)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}

	result := decoder.Feed(capture)
	flushed := decoder.Flush()
	result.Payloads = append(result.Payloads, flushed.Payloads...)
	result.Errors = append(result.Errors, flushed.Errors...)

	if len(result.Errors) != 0 {
		t.Fatalf("decode errors = %v", result.Errors)
	}
	if len(result.Payloads) != 10 {
		t.Fatalf("payload count = %d, want 10", len(result.Payloads))
	}
	assertContainsPayload(t, result.Payloads, `"type":"msg"`, `"msg":"Direct message{663"`)
	assertContainsPayload(t, result.Payloads, `"type":"msg"`, `"msg":"QQ0AAA-10:ack663"`)
	assertContainsPayload(t, result.Payloads, `"type":"pos"`, `"src_type":"node"`)
	assertContainsPayload(t, result.Payloads, `"type":"tele"`, `"src_type":"lora"`)

	for _, payload := range result.Payloads {
		if bytes.Contains(payload, []byte("REDACTED")) ||
			bytes.Contains(payload, []byte("[HEAP]")) ||
			bytes.Contains(payload, []byte("GPS<")) ||
			bytes.Contains(payload, []byte("BLE Connected")) {
			t.Fatalf("diagnostic extracted as payload: %q", payload)
		}
	}
}

func TestDecoderProducesSamePayloadsForArbitraryChunking(t *testing.T) {
	stream := []byte(
		`[EXT] Out: {"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"one"}` + "\r\n" +
			`[12:34:56][EXT] Tele-Out: {"type":"tele","src":"QQ1BBB-2","temp1":20}` + "\n" +
			`[EXT] Out: {"type":"msg","src":"QQ1CCC-3","dst":"222","msg":"three"}` + "\n",
	)
	want := decodeChunks(t, stream, len(stream))

	for chunkSize := 1; chunkSize <= len(stream); chunkSize++ {
		got := decodeChunks(t, stream, chunkSize)
		if !equalPayloads(got, want) {
			t.Fatalf("chunk size %d payloads = %q, want %q", chunkSize, got, want)
		}
	}
}

func TestDecoderHandlesRecords(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPayload string
		wantError   error
	}{
		{
			name:        "braces and suffix inside message",
			input:       `[EXT] Out: {"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"a { brace } Len: \", ok"} Len: 99` + "\n",
			wantPayload: `{"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"a { brace } Len: \", ok"}`,
		},
		{
			name:        "balanced malformed JSON remains available for forwarding",
			input:       `[EXT] Out: {invalid}` + "\n",
			wantPayload: `{invalid}`,
		},
		{
			name:      "missing object",
			input:     `[EXT] Out: no object` + "\n",
			wantError: ErrMalformedExtRecord,
		},
		{
			name:      "incomplete object",
			input:     `[EXT] Out: {"type":"msg"` + "\n",
			wantError: ErrMalformedExtRecord,
		},
		{
			name:  "incoming diagnostic ignored",
			input: `[EXT] Inc: {"type":"msg","src":"QQ1AAA-1"}` + "\n",
		},
		{
			name:  "echoed command cannot inject Ext marker",
			input: `::message [EXT] Out: {"type":"msg","src":"QQ1AAA-1"}` + "\n",
		},
		{
			name:  "invalid capture prefix ignored",
			input: `[not-time][EXT] Out: {"type":"msg","src":"QQ1AAA-1"}` + "\n",
		},
		{
			name:  "blank line ignored",
			input: "\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder, err := NewDecoder(1024)
			if err != nil {
				t.Fatalf("NewDecoder() error = %v", err)
			}
			result := decoder.Feed([]byte(tt.input))
			if tt.wantError != nil {
				if len(result.Errors) != 1 || !errors.Is(result.Errors[0], tt.wantError) {
					t.Fatalf("errors = %v, want %v", result.Errors, tt.wantError)
				}
				return
			}
			if len(result.Errors) != 0 {
				t.Fatalf("errors = %v", result.Errors)
			}
			if tt.wantPayload == "" {
				if len(result.Payloads) != 0 {
					t.Fatalf("payloads = %q, want none", result.Payloads)
				}
				return
			}
			if len(result.Payloads) != 1 || string(result.Payloads[0]) != tt.wantPayload {
				t.Fatalf("payloads = %q, want %q", result.Payloads, tt.wantPayload)
			}
		})
	}
}

func TestDecoderRejectsOversizedRecordAndRecovers(t *testing.T) {
	decoder, err := NewDecoder(32)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	oversized := bytes.Repeat([]byte{'x'}, 33)
	input := append(oversized, '\n')
	input = append(input, []byte(`[EXT] Out: {}`+"\n")...)

	result := decoder.Feed(input)
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], ErrRecordTooLong) {
		t.Fatalf("errors = %v, want ErrRecordTooLong", result.Errors)
	}
	if len(result.Payloads) != 1 || string(result.Payloads[0]) != `{}` {
		t.Fatalf("payloads = %q, want {}", result.Payloads)
	}
	if decoder.BufferedBytes() != 0 {
		t.Fatalf("buffered bytes = %d, want 0", decoder.BufferedBytes())
	}
}

func TestDecoderFlushesFinalRecordAndDiscardState(t *testing.T) {
	decoder, err := NewDecoder(64)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	result := decoder.Feed([]byte(`[EXT] Out: {"type":"msg"}`))
	if len(result.Payloads) != 0 {
		t.Fatalf("Feed() payloads = %q, want none before flush", result.Payloads)
	}
	result = decoder.Flush()
	if len(result.Payloads) != 1 || string(result.Payloads[0]) != `{"type":"msg"}` {
		t.Fatalf("Flush() payloads = %q", result.Payloads)
	}
	if result := decoder.Flush(); len(result.Payloads) != 0 || len(result.Errors) != 0 {
		t.Fatalf("second Flush() = %+v, want empty", result)
	}

	decoder, err = NewDecoder(4)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	result = decoder.Feed([]byte("12345"))
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], ErrRecordTooLong) {
		t.Fatalf("overflow errors = %v", result.Errors)
	}
	if result := decoder.Flush(); len(result.Payloads) != 0 || len(result.Errors) != 0 {
		t.Fatalf("discard Flush() = %+v, want empty", result)
	}
}

func TestNewDecoderRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []int{0, -1} {
		if _, err := NewDecoder(limit); !errors.Is(err, ErrInvalidRecordLimit) {
			t.Fatalf("NewDecoder(%d) error = %v, want ErrInvalidRecordLimit", limit, err)
		}
	}
}

func FuzzDecoder(f *testing.F) {
	seeds := [][]byte{
		[]byte(`[EXT] Out: {"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"hello"}` + "\n"),
		[]byte(`[12:34:56][EXT] Tele-Out: {"type":"tele","src":"QQ1BBB-2","temp1":0}` + "\r\n"),
		[]byte(`[EXT] Out: {"msg":"braces { } and quote \""}` + "\n"),
		[]byte(`[EXT] Inc: {"type":"msg"}` + "\n"),
		bytes.Repeat([]byte{'x'}, 65),
		{0, 1, 2, '\n'},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		const limit = 64
		decoder, err := NewDecoder(limit)
		if err != nil {
			t.Fatalf("NewDecoder() error = %v", err)
		}
		for _, value := range input {
			result := decoder.Feed([]byte{value})
			assertBoundedPayloads(t, result.Payloads, limit)
			if decoder.BufferedBytes() > limit {
				t.Fatalf("buffered bytes = %d, limit %d", decoder.BufferedBytes(), limit)
			}
		}
		result := decoder.Flush()
		assertBoundedPayloads(t, result.Payloads, limit)
	})
}

func decodeChunks(t *testing.T, stream []byte, chunkSize int) [][]byte {
	t.Helper()
	decoder, err := NewDecoder(1024)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	var payloads [][]byte
	for start := 0; start < len(stream); start += chunkSize {
		end := min(start+chunkSize, len(stream))
		result := decoder.Feed(stream[start:end])
		if len(result.Errors) != 0 {
			t.Fatalf("decode errors = %v", result.Errors)
		}
		payloads = append(payloads, result.Payloads...)
	}
	result := decoder.Flush()
	if len(result.Errors) != 0 {
		t.Fatalf("flush errors = %v", result.Errors)
	}
	return append(payloads, result.Payloads...)
}

func equalPayloads(left, right [][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}

func assertContainsPayload(t *testing.T, payloads [][]byte, fragments ...string) {
	t.Helper()
	for _, payload := range payloads {
		matches := true
		for _, fragment := range fragments {
			if !bytes.Contains(payload, []byte(fragment)) {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("no payload contains %q", fragments)
}

func assertBoundedPayloads(t *testing.T, payloads [][]byte, limit int) {
	t.Helper()
	for _, payload := range payloads {
		if len(payload) > limit {
			t.Fatalf("payload length = %d, limit %d", len(payload), limit)
		}
		if len(payload) < 2 || payload[0] != '{' || payload[len(payload)-1] != '}' {
			t.Fatalf("invalid payload framing: %q", payload)
		}
	}
}
