package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportReceiveLogImportsJSONL(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	first := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	writeReceiveLogFile(t, filepath.Join(dir, "received.20260516.jsonl"),
		`{"received_at":"`+first.Format(time.RFC3339Nano)+`","remote_addr":"127.0.0.1:1799","bytes":15,"raw":"{\"type\":\"msg\"}","packet_type":"msg"}`+"\n"+
			`{"received_at":"`+second.Format(time.RFC3339Nano)+`","remote_addr":"127.0.0.2:1799","bytes":20,"raw":"bad","parse_error":"broken"}`+"\n",
	)

	if err := db.ImportReceiveLog(ctx, dir); err != nil {
		t.Fatalf("ImportReceiveLog() error = %v", err)
	}

	var count int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM receive_log`).Scan(&count); err != nil {
		t.Fatalf("count receive_log: %v", err)
	}
	if count != 2 {
		t.Fatalf("receive_log count = %d, want 2", count)
	}
	assertImportRecorded(t, db.SQL(), receiveLogImportSource, 2)
}

func TestImportReceiveLogMissingDirRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportReceiveLog(ctx, filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("ImportReceiveLog() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), receiveLogImportSource, 0)
}

func TestImportReceiveLogInvalidJSONFailsWithFileLine(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "received.20260516.jsonl")
	writeReceiveLogFile(t, path, "not-json\n")

	err := db.ImportReceiveLog(ctx, dir)
	if err == nil {
		t.Fatal("ImportReceiveLog() error = nil, want error")
	}
	if !strings.Contains(err.Error(), path+":1") {
		t.Fatalf("error = %v, want file:line", err)
	}
}

func TestImportReceiveLogIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	receivedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "received.20260516.jsonl")
	writeReceiveLogFile(t, path, `{"received_at":"`+receivedAt.Format(time.RFC3339Nano)+`","remote_addr":"one","bytes":1,"raw":"one"}`+"\n")

	if err := db.ImportReceiveLog(ctx, dir); err != nil {
		t.Fatalf("first ImportReceiveLog() error = %v", err)
	}
	writeReceiveLogFile(t, path, `{"received_at":"`+receivedAt.Format(time.RFC3339Nano)+`","remote_addr":"two","bytes":2,"raw":"two"}`+"\n")
	if err := db.ImportReceiveLog(ctx, dir); err != nil {
		t.Fatalf("second ImportReceiveLog() error = %v", err)
	}

	var count int
	var remoteAddr string
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*), MAX(remote_addr) FROM receive_log`).Scan(&count, &remoteAddr); err != nil {
		t.Fatalf("query receive_log: %v", err)
	}
	if count != 1 || remoteAddr != "one" {
		t.Fatalf("count=%d remote_addr=%q, want 1/one", count, remoteAddr)
	}
}

func writeReceiveLogFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
