package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const chatReadsImportSource = "chat_reads"

type chatReadImportEntry struct {
	LastRead time.Time `json:"lastRead"`
}

func (db *DB) ImportChatReads(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, chatReadsImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	entries, sourceInfo, err := readChatReads(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for convID, entry := range entries {
			if entry.LastRead.IsZero() {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO chat_reads(conversation_id, last_read)
				VALUES (?, ?)
				ON CONFLICT(conversation_id) DO UPDATE SET last_read = excluded.last_read
			`, convID, entry.LastRead.UTC().Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("insert chat read %s: %w", convID, err)
			}
		}
		return recordImport(ctx, tx, chatReadsImportSource, sourceInfo, len(entries))
	})
}

func readChatReads(path string) (map[string]chatReadImportEntry, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]chatReadImportEntry{}, importSourceInfo{path: path}, nil
		}
		return nil, importSourceInfo{}, fmt.Errorf("open chat reads import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("stat chat reads import %s: %w", path, err)
	}

	var entries map[string]chatReadImportEntry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("decode chat reads import %s: %w", path, err)
	}
	if entries == nil {
		entries = map[string]chatReadImportEntry{}
	}
	return entries, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}
