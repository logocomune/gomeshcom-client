package logfmt

import (
	"bytes"
	"context"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

var fixedTime = time.Date(2026, 5, 23, 6, 42, 10, 0, time.UTC)

func newRecord(level slog.Level, msg string, attrs ...slog.Attr) slog.Record {
	r := slog.NewRecord(fixedTime, level, msg, 0)
	r.AddAttrs(attrs...)
	return r
}

func TestHandleFormatsLine(t *testing.T) {
	tests := []struct {
		name    string
		record  slog.Record
		want    []string
		notWant []string
	}{
		{
			name:   "timestamp and message",
			record: newRecord(slog.LevelInfo, "server started"),
			want:   []string{"2026-05-23 06:42:10", "INFO ", "server started"},
		},
		{
			name:   "debug label",
			record: newRecord(slog.LevelDebug, "trace"),
			want:   []string{"DEBUG"},
		},
		{
			name:   "warn label padded",
			record: newRecord(slog.LevelWarn, "slow"),
			want:   []string{"WARN "},
		},
		{
			name:   "error label",
			record: newRecord(slog.LevelError, "crash"),
			want:   []string{"ERROR"},
		},
		{
			name:   "string attr raw",
			record: newRecord(slog.LevelInfo, "test", slog.String("addr", "127.0.0.1:8080")),
			want:   []string{"addr=127.0.0.1:8080"},
		},
		{
			name:   "string attr with spaces quoted",
			record: newRecord(slog.LevelInfo, "test", slog.String("msg", "hello world")),
			want:   []string{`msg="hello world"`},
		},
		{
			name:   "empty string attr quoted",
			record: newRecord(slog.LevelInfo, "test", slog.String("key", "")),
			want:   []string{`key=""`},
		},
		{
			name:   "int attr",
			record: newRecord(slog.LevelInfo, "test", slog.Int("status", 200)),
			want:   []string{"status=200"},
		},
		{
			name:   "bool attr",
			record: newRecord(slog.LevelInfo, "test", slog.Bool("enabled", true)),
			want:   []string{"enabled=true"},
		},
		{
			name:   "duration attr",
			record: newRecord(slog.LevelInfo, "test", slog.Duration("latency", 42*time.Millisecond)),
			want:   []string{"latency=42ms"},
		},
		{
			name:   "multiple attrs",
			record: newRecord(slog.LevelInfo, "req", slog.String("method", "GET"), slog.Int("status", 200)),
			want:   []string{"method=GET", "status=200"},
		},
		{
			name:   "line ends with newline",
			record: newRecord(slog.LevelInfo, "x"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := New(&buf, slog.LevelDebug)

			if err := h.Handle(context.Background(), tt.record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			line := buf.String()

			if tt.name == "line ends with newline" {
				if !strings.HasSuffix(line, "\n") {
					t.Fatalf("line = %q does not end with newline", line)
				}
				return
			}

			for _, want := range tt.want {
				if !strings.Contains(line, want) {
					t.Errorf("line = %q, missing %q", line, want)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(line, notWant) {
					t.Errorf("line = %q, unexpected %q", line, notWant)
				}
			}
		})
	}
}

func TestEnabledFiltersLevels(t *testing.T) {
	tests := []struct {
		name        string
		minLevel    slog.Level
		recordLevel slog.Level
		wantEnabled bool
	}{
		{"debug passes debug", slog.LevelDebug, slog.LevelDebug, true},
		{"info passes info", slog.LevelInfo, slog.LevelInfo, true},
		{"info blocks debug", slog.LevelInfo, slog.LevelDebug, false},
		{"warn passes error", slog.LevelWarn, slog.LevelError, true},
		{"error blocks warn", slog.LevelError, slog.LevelWarn, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(new(bytes.Buffer), tt.minLevel)
			if got := h.Enabled(context.Background(), tt.recordLevel); got != tt.wantEnabled {
				t.Fatalf("Enabled(%v) = %v, want %v", tt.recordLevel, got, tt.wantEnabled)
			}
		})
	}
}

func TestWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, slog.LevelDebug).WithAttrs([]slog.Attr{slog.String("service", "meshcom")})

	r := newRecord(slog.LevelInfo, "started")
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	line := buf.String()
	if !strings.Contains(line, "service=meshcom") {
		t.Fatalf("line = %q, want service=meshcom", line)
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, slog.LevelDebug).
		WithGroup("net").
		WithAttrs([]slog.Attr{slog.String("addr", "127.0.0.1")})

	r := newRecord(slog.LevelInfo, "connected", slog.String("proto", "tcp"))
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	line := buf.String()
	if !strings.Contains(line, "net.addr=127.0.0.1") {
		t.Fatalf("line = %q, want net.addr=127.0.0.1", line)
	}
	if !strings.Contains(line, "net.proto=tcp") {
		t.Fatalf("line = %q, want net.proto=tcp", line)
	}
}

func TestWithGroupEmpty(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, slog.LevelDebug)
	h2 := h.WithGroup("")
	if h2 != h {
		t.Fatal("WithGroup(\"\") should return the same handler")
	}
}

func TestWithAttrsDoesNotMutateOriginal(t *testing.T) {
	var buf bytes.Buffer
	base := New(&buf, slog.LevelDebug)
	child := base.WithAttrs([]slog.Attr{slog.String("child", "yes")})

	r := newRecord(slog.LevelInfo, "base log")
	if err := base.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if strings.Contains(buf.String(), "child=yes") {
		t.Fatalf("base handler output should not contain child attr: %q", buf.String())
	}

	buf.Reset()
	r2 := newRecord(slog.LevelInfo, "child log")
	if err := child.Handle(context.Background(), r2); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !strings.Contains(buf.String(), "child=yes") {
		t.Fatalf("child handler output should contain child attr: %q", buf.String())
	}
}

func TestHandleConcurrent(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, slog.LevelDebug)

	const n = 100
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			r := newRecord(slog.LevelInfo, "concurrent")
			_ = h.Handle(context.Background(), r)
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}

	lines := strings.Count(buf.String(), "\n")
	if lines != n {
		t.Fatalf("expected %d newlines, got %d", n, lines)
	}
}

func TestValueFormattingRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	f := func(s string) bool {
		var sb strings.Builder
		appendValue(&sb, slog.StringValue(s))
		result := sb.String()
		if strings.HasPrefix(result, `"`) {
			unquoted, err := strconv.Unquote(result)
			return err == nil && unquoted == s
		}
		return result == s
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 1000, Rand: rng}); err != nil {
		t.Errorf("value formatting round-trip failed: %v", err)
	}
}
