package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const httpSessionsImportSource = "http_sessions"

type httpSessionsImportFile struct {
	Sessions map[string]time.Time `json:"sessions"`
}

func (db *DB) ImportHTTPSessions(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, httpSessionsImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	sessions, sourceInfo, err := readHTTPSessions(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for tokenHash, expiresAt := range sessions {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO http_sessions(token_hash, expires_at)
				VALUES (?, ?)
				ON CONFLICT(token_hash) DO UPDATE SET expires_at = excluded.expires_at
			`, tokenHash, expiresAt.UTC().Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("insert http session: %w", err)
			}
		}
		return recordImport(ctx, tx, httpSessionsImportSource, sourceInfo, len(sessions))
	})
}

func readHTTPSessions(path string) (map[string]time.Time, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]time.Time{}, importSourceInfo{path: path}, nil
		}
		return nil, importSourceInfo{}, fmt.Errorf("open http sessions import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("stat http sessions import %s: %w", path, err)
	}

	var persisted httpSessionsImportFile
	if err := json.NewDecoder(file).Decode(&persisted); err != nil {
		return map[string]time.Time{}, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
	}

	now := time.Now().UTC()
	sessions := make(map[string]time.Time, len(persisted.Sessions))
	for tokenHash, expiresAt := range persisted.Sessions {
		if tokenHash != "" && expiresAt.After(now) {
			sessions[tokenHash] = expiresAt
		}
	}
	return sessions, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}
