package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportChatReadsImportsLastRead(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "msg_idx.json")
	lastRead := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	writeFile(t, path, `{"P_broadcast":{"lastMsgReceived":"2026-05-16T11:00:00Z","lastRead":"`+lastRead.Format(time.RFC3339Nano)+`","unreadCount":3,"lastMsg":"hello"}}`)

	if err := db.ImportChatReads(ctx, path); err != nil {
		t.Fatalf("ImportChatReads() error = %v", err)
	}

	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT last_read FROM chat_reads WHERE conversation_id = 'P_broadcast'`).Scan(&got); err != nil {
		t.Fatalf("query chat_reads: %v", err)
	}
	if got != lastRead.Format(time.RFC3339Nano) {
		t.Fatalf("last_read = %q, want %q", got, lastRead.Format(time.RFC3339Nano))
	}
	assertImportRecorded(t, db.SQL(), chatReadsImportSource, 1)
}

func TestImportChatReadsMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportChatReads(ctx, filepath.Join(t.TempDir(), "msg_idx.json")); err != nil {
		t.Fatalf("ImportChatReads() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), chatReadsImportSource, 0)
}

func TestImportChatReadsInvalidJSONFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "msg_idx.json")
	writeFile(t, path, "not-json")
	err := db.ImportChatReads(ctx, path)
	if err == nil {
		t.Fatal("ImportChatReads() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode chat reads import") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestImportChatReadsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "msg_idx.json")
	first := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	writeFile(t, path, `{"P_broadcast":{"lastRead":"`+first.Format(time.RFC3339Nano)+`"}}`)
	if err := db.ImportChatReads(ctx, path); err != nil {
		t.Fatalf("first ImportChatReads() error = %v", err)
	}
	writeFile(t, path, `{"P_broadcast":{"lastRead":"`+second.Format(time.RFC3339Nano)+`"}}`)
	if err := db.ImportChatReads(ctx, path); err != nil {
		t.Fatalf("second ImportChatReads() error = %v", err)
	}
	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT last_read FROM chat_reads WHERE conversation_id = 'P_broadcast'`).Scan(&got); err != nil {
		t.Fatalf("query chat_reads: %v", err)
	}
	if got != first.Format(time.RFC3339Nano) {
		t.Fatalf("last_read = %q, want first import", got)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
