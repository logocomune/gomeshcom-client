package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportChannelShowImportsJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "channel_show.json")
	writeFile(t, path, `{"mode":"allowlist","channels":["*","222","222"]}`)
	lastAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := db.SQL().ExecContext(ctx, `INSERT INTO chats_public(conversation_id, kind, channel, received_at, dst, msg) VALUES ('P_222', 'channel', '222', ?, '222', 'hello')`, lastAt); err != nil {
		t.Fatalf("insert chat row: %v", err)
	}

	if err := db.ImportChannelShow(ctx, path); err != nil {
		t.Fatalf("ImportChannelShow() error = %v", err)
	}

	var mode string
	if err := db.SQL().QueryRowContext(ctx, `SELECT mode FROM channel_show WHERE id = 1`).Scan(&mode); err != nil {
		t.Fatalf("query channel_show: %v", err)
	}
	if mode != "allowlist" {
		t.Fatalf("mode = %q, want allowlist", mode)
	}
	var lastMessageAt string
	if err := db.SQL().QueryRowContext(ctx, `SELECT last_message_at FROM channel_show_channels WHERE channel = '222'`).Scan(&lastMessageAt); err != nil {
		t.Fatalf("query channel_show_channels: %v", err)
	}
	if lastMessageAt != lastAt {
		t.Fatalf("last_message_at = %q, want %q", lastMessageAt, lastAt)
	}
	assertImportRecorded(t, db.SQL(), channelShowImportSource, 2)
}

func TestImportChannelShowMissingFileRecordsDefault(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportChannelShow(ctx, filepath.Join(t.TempDir(), "channel_show.json")); err != nil {
		t.Fatalf("ImportChannelShow() error = %v", err)
	}
	var mode string
	if err := db.SQL().QueryRowContext(ctx, `SELECT mode FROM channel_show WHERE id = 1`).Scan(&mode); err != nil {
		t.Fatalf("query channel_show: %v", err)
	}
	if mode != "all" {
		t.Fatalf("mode = %q, want all", mode)
	}
	assertImportRecorded(t, db.SQL(), channelShowImportSource, 0)
}

func TestImportChannelShowInvalidJSONFails(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "channel_show.json")
	writeFile(t, path, "not-json")
	err := db.ImportChannelShow(ctx, path)
	if err == nil {
		t.Fatal("ImportChannelShow() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "decode channel show import") {
		t.Fatalf("error = %v, want decode context", err)
	}
}

func TestImportChannelShowIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "channel_show.json")
	writeFile(t, path, `{"mode":"allowlist","channels":["222"]}`)
	if err := db.ImportChannelShow(ctx, path); err != nil {
		t.Fatalf("first ImportChannelShow() error = %v", err)
	}
	writeFile(t, path, `{"mode":"all","channels":[]}`)
	if err := db.ImportChannelShow(ctx, path); err != nil {
		t.Fatalf("second ImportChannelShow() error = %v", err)
	}
	var mode string
	if err := db.SQL().QueryRowContext(ctx, `SELECT mode FROM channel_show WHERE id = 1`).Scan(&mode); err != nil {
		t.Fatalf("query channel_show: %v", err)
	}
	if mode != "allowlist" {
		t.Fatalf("mode = %q, want first import allowlist", mode)
	}
}
