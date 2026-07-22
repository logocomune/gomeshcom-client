package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
)

func TestImportChatHistoryImportsPublicAndDM(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	writeChatJSONL(t, filepath.Join(dir, "P_broadcast.jsonl"), []chatlog.Record{{ReceivedAt: now, Src: "SRC-1", Dst: "*", MsgID: "PUB1", Msg: "hello", RSSI: -70, SNR: 8}})
	writeChatJSONL(t, filepath.Join(dir, "P_123.jsonl"), []chatlog.Record{{ReceivedAt: now, Src: "SRC-1", Dst: "123", Msg: "channel"}})
	writeChatJSONL(t, filepath.Join(dir, "DM_QQ0QQ_QQ1ABC-1.jsonl"), []chatlog.Record{{ReceivedAt: now, Src: "QQ1ABC-1", Dst: "QQ0QQ-1", MsgID: "DM1", Msg: "dm"}})
	writeChatJSONL(t, filepath.Join(dir, "ignored.jsonl"), []chatlog.Record{{ReceivedAt: now, Msg: "ignore"}})

	if err := db.ImportChatHistory(ctx, dir); err != nil {
		t.Fatalf("ImportChatHistory() error = %v", err)
	}

	var publicCount, dmCount int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM chats_public`).Scan(&publicCount); err != nil {
		t.Fatalf("count chats_public: %v", err)
	}
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM chats_dm`).Scan(&dmCount); err != nil {
		t.Fatalf("count chats_dm: %v", err)
	}
	if publicCount != 2 || dmCount != 1 {
		t.Fatalf("public/dm count = %d/%d, want 2/1", publicCount, dmCount)
	}

	var kind string
	var channel any
	if err := db.SQL().QueryRowContext(ctx, `SELECT kind, channel FROM chats_public WHERE conversation_id = 'P_123'`).Scan(&kind, &channel); err != nil {
		t.Fatalf("query channel row: %v", err)
	}
	if kind != "channel" || channel.(string) != "123" {
		t.Fatalf("kind/channel = %q/%v, want channel/123", kind, channel)
	}
	assertImportRecorded(t, db.SQL(), chatHistoryImportSource, 3)
}

func TestImportChatHistoryMissingDirRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportChatHistory(ctx, filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("ImportChatHistory() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), chatHistoryImportSource, 0)
}

func TestImportChatHistoryInvalidJSONFailsWithFileLine(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	path := filepath.Join(dir, "P_broadcast.jsonl")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := db.ImportChatHistory(ctx, dir)
	if err == nil {
		t.Fatal("ImportChatHistory() error = nil, want error")
	}
	if !strings.Contains(err.Error(), path+":1") {
		t.Fatalf("error = %v, want file:line", err)
	}
}

func TestImportChatHistoryIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	dir := t.TempDir()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(dir, "P_broadcast.jsonl")
	writeChatJSONL(t, path, []chatlog.Record{{ReceivedAt: now, Dst: "*", Msg: "first"}})
	if err := db.ImportChatHistory(ctx, dir); err != nil {
		t.Fatalf("first ImportChatHistory() error = %v", err)
	}
	writeChatJSONL(t, path, []chatlog.Record{{ReceivedAt: now, Dst: "*", Msg: "second"}})
	if err := db.ImportChatHistory(ctx, dir); err != nil {
		t.Fatalf("second ImportChatHistory() error = %v", err)
	}

	var count int
	var msg string
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*), MAX(msg) FROM chats_public`).Scan(&count, &msg); err != nil {
		t.Fatalf("query chats_public: %v", err)
	}
	if count != 1 || msg != "first" {
		t.Fatalf("count/msg = %d/%q, want 1/first", count, msg)
	}
}

func writeChatJSONL(t *testing.T, path string, records []chatlog.Record) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
}
