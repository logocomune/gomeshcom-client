package storage

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
)

const chatHistoryImportSource = "chat_history"

func (db *DB) ImportChatHistory(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, chatHistoryImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	paths, err := chatHistoryPaths(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		recordCount := 0
		for _, sourcePath := range paths {
			conversationID := strings.TrimSuffix(filepath.Base(sourcePath), ".jsonl")
			count, err := importChatFile(ctx, tx, sourcePath, conversationID)
			if err != nil {
				return err
			}
			recordCount += count
		}
		return recordImport(ctx, tx, chatHistoryImportSource, importSourceInfo{path: path}, recordCount)
	})
}

func chatHistoryPaths(path string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(path, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("list chat history import files: %w", err)
	}
	validPaths := paths[:0]
	for _, sourcePath := range paths {
		conversationID := strings.TrimSuffix(filepath.Base(sourcePath), ".jsonl")
		if chatlog.ValidConversationID(conversationID) {
			validPaths = append(validPaths, sourcePath)
		}
	}
	sort.Strings(validPaths)
	return validPaths, nil
}

func importChatFile(ctx context.Context, tx *sql.Tx, path string, conversationID string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open chat history import %s: %w", path, err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		var record chatlog.Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return 0, fmt.Errorf("decode chat history import %s:%d: %w", path, line, err)
		}
		if err := insertChatRecord(ctx, tx, conversationID, record); err != nil {
			return 0, err
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan chat history import %s: %w", path, err)
	}
	return count, nil
}

func insertChatRecord(ctx context.Context, tx *sql.Tx, conversationID string, record chatlog.Record) error {
	if strings.HasPrefix(conversationID, "DM_") {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO chats_dm(
				conversation_id, msg_id, received_at, src, src_type, dst, msg, rssi, snr, direction, delivery_status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, conversationID, nullableImportString(record.MsgID), formatRecordTime(record.ReceivedAt), nullableImportString(record.Src), nullableImportString(record.SrcType), record.Dst, record.Msg, nullableInt(record.RSSI), nullableInt(record.SNR), nullableImportString(record.Direction), nullableImportString(record.DeliveryStatus))
		if err != nil {
			return fmt.Errorf("insert dm chat record %s: %w", conversationID, err)
		}
		return nil
	}

	kind, channel := publicConversationKind(conversationID)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO chats_public(
			conversation_id, kind, channel, msg_id, received_at, src, src_type, dst, msg, rssi, snr
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, conversationID, kind, channel, nullableImportString(record.MsgID), formatRecordTime(record.ReceivedAt), nullableImportString(record.Src), nullableImportString(record.SrcType), record.Dst, record.Msg, nullableInt(record.RSSI), nullableInt(record.SNR))
	if err != nil {
		return fmt.Errorf("insert public chat record %s: %w", conversationID, err)
	}
	return nil
}

func publicConversationKind(conversationID string) (string, any) {
	if conversationID == "P_broadcast" {
		return "broadcast", nil
	}
	return "channel", strings.TrimPrefix(conversationID, "P_")
}

func formatRecordTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
