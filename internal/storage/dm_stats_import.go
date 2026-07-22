package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const dmStatsImportSource = "dm_stats"

type dmStatsImportEntry struct {
	Sent int `json:"sent"`
	Ack  int `json:"ack"`
}

func (db *DB) ImportDMStats(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, dmStatsImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	entries, sourceInfo, err := readDMStats(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for callsign, entry := range entries {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO dm_stats(callsign, sent, ack)
				VALUES (?, ?, ?)
				ON CONFLICT(callsign) DO UPDATE SET sent = excluded.sent, ack = excluded.ack
			`, callsign, entry.Sent, entry.Ack); err != nil {
				return fmt.Errorf("insert dm stats %s: %w", callsign, err)
			}
		}
		return recordImport(ctx, tx, dmStatsImportSource, sourceInfo, len(entries))
	})
}

func readDMStats(path string) (map[string]dmStatsImportEntry, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]dmStatsImportEntry{}, importSourceInfo{path: path}, nil
		}
		return nil, importSourceInfo{}, fmt.Errorf("open dm stats import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("stat dm stats import %s: %w", path, err)
	}

	var entries map[string]dmStatsImportEntry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("decode dm stats import %s: %w", path, err)
	}
	if entries == nil {
		entries = map[string]dmStatsImportEntry{}
	}
	return entries, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}
