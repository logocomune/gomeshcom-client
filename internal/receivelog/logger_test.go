package receivelog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerAppend(t *testing.T) {
	dir := t.TempDir()
	logger := New(Config{Enabled: true, Path: dir})

	record := Record{
		ReceivedAt: time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		RemoteAddr: "127.0.0.1:1799",
		Bytes:      15,
		Raw:        `{"type":"msg"}`,
		PacketType: "msg",
	}

	if err := logger.Append(record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	lines := readJSONLines(t, filepath.Join(dir, "received.20260516.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}

	var got Record
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if got.PacketType != "msg" {
		t.Fatalf("packet type = %q, want msg", got.PacketType)
	}
}

func TestLoggerDisabled(t *testing.T) {
	dir := t.TempDir()
	logger := New(Config{Enabled: false, Path: dir})

	if err := logger.Append(Record{Raw: `{"type":"msg"}`}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entry count = %d, want 0", len(entries))
	}
}

func TestLoggerAppendUsesRecordDate(t *testing.T) {
	dir := t.TempDir()
	logger := New(Config{Enabled: true, Path: dir})

	if err := logger.Append(Record{
		ReceivedAt: time.Date(2026, 5, 16, 23, 59, 0, 0, time.UTC),
		Raw:        `{"type":"msg","msg":"first"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append first record: %v", err)
	}
	if err := logger.Append(Record{
		ReceivedAt: time.Date(2026, 5, 17, 0, 1, 0, 0, time.UTC),
		Raw:        `{"type":"msg","msg":"second"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append second record: %v", err)
	}

	if lines := readJSONLines(t, filepath.Join(dir, "received.20260516.jsonl")); len(lines) != 1 {
		t.Fatalf("20260516 line count = %d, want 1", len(lines))
	}
	if lines := readJSONLines(t, filepath.Join(dir, "received.20260517.jsonl")); len(lines) != 1 {
		t.Fatalf("20260517 line count = %d, want 1", len(lines))
	}
}

func TestLoggerReadSince(t *testing.T) {
	dir := t.TempDir()
	logger := New(Config{Enabled: true, Path: dir})
	cutoff := dayStart(time.Now().UTC()).Add(12 * time.Hour)

	if err := logger.Append(Record{
		ReceivedAt: cutoff.Add(-time.Second),
		Raw:        `{"type":"msg","msg":"old"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append old record: %v", err)
	}
	if err := logger.Append(Record{
		ReceivedAt: cutoff,
		Raw:        `{"type":"msg","msg":"new"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append new record: %v", err)
	}

	records, err := logger.ReadSince(cutoff)
	if err != nil {
		t.Fatalf("ReadSince() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	if records[0].Raw != `{"type":"msg","msg":"new"}` {
		t.Fatalf("raw = %q", records[0].Raw)
	}
}

func TestLoggerPrunesRecordsOutsideRetentionDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	logger := New(Config{Enabled: true, Path: dir, RetentionDays: 365})

	if err := logger.Append(Record{
		ReceivedAt: dayStart(now).AddDate(0, 0, -365),
		Raw:        `{"type":"msg","msg":"expired"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append expired record: %v", err)
	}
	if err := logger.Append(Record{
		ReceivedAt: dayStart(now).AddDate(0, 0, -364),
		Raw:        `{"type":"msg","msg":"kept"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append kept record: %v", err)
	}
	if err := logger.Append(Record{
		ReceivedAt: now,
		Raw:        `{"type":"msg","msg":"fresh"}`,
		PacketType: "msg",
	}); err != nil {
		t.Fatalf("append fresh record: %v", err)
	}

	expiredPath := filepath.Join(dir, "received."+dayStart(now).AddDate(0, 0, -365).Format(fileDateLayout)+".jsonl")
	if _, err := os.Stat(expiredPath); !os.IsNotExist(err) {
		t.Fatalf("expired file stat error = %v, want not exist", err)
	}

	keptPath := filepath.Join(dir, "received."+dayStart(now).AddDate(0, 0, -364).Format(fileDateLayout)+".jsonl")
	if lines := readJSONLines(t, keptPath); len(lines) != 1 {
		t.Fatalf("kept file line count = %d, want 1", len(lines))
	}

	lines := readJSONLines(t, filepath.Join(dir, "received."+now.Format(fileDateLayout)+".jsonl"))
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1", len(lines))
	}
	if lines[0] != `{"received_at":"`+now.Format(time.RFC3339Nano)+`","remote_addr":"","bytes":0,"raw":"{\"type\":\"msg\",\"msg\":\"fresh\"}","packet_type":"msg"}` {
		t.Fatalf("line = %q", lines[0])
	}
}

func TestLoggerValidate(t *testing.T) {
	// Missing path
	l1 := New(Config{Enabled: true})
	if err := l1.Append(Record{}); err == nil {
		t.Error("append with empty path: want error, got nil")
	}

	// Negative retention
	l2 := New(Config{Enabled: true, Path: "/tmp", RetentionDays: -1})
	if err := l2.Append(Record{}); err == nil {
		t.Error("append with negative retention: want error, got nil")
	}
}

func TestLoggerReadSinceEmpty(t *testing.T) {
	dir := t.TempDir()
	logger := New(Config{Enabled: true, Path: dir})
	
	records, err := logger.ReadSince(time.Now().UTC())
	if err != nil {
		t.Fatalf("ReadSince empty: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("ReadSince empty: got %d records, want 0", len(records))
	}
}

func readJSONLines(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}

	return lines
}
