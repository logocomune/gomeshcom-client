package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/callsign"
)

const stationIdentityImportSource = "station_identity"

type stationIdentityImportRecord struct {
	Callsign string `json:"callsign"`
}

func (db *DB) ImportStationIdentity(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, stationIdentityImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	call, sourceInfo, err := readStationIdentity(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		count := 0
		if call != "" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO station_identity(id, callsign)
				VALUES (1, ?)
				ON CONFLICT(id) DO UPDATE SET callsign = excluded.callsign
			`, call); err != nil {
				return fmt.Errorf("insert station identity: %w", err)
			}
			count = 1
		}
		return recordImport(ctx, tx, stationIdentityImportSource, sourceInfo, count)
	})
}

func readStationIdentity(path string) (string, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", importSourceInfo{path: path}, nil
		}
		return "", importSourceInfo{}, fmt.Errorf("open station identity import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", importSourceInfo{}, fmt.Errorf("stat station identity import %s: %w", path, err)
	}

	var record stationIdentityImportRecord
	if err := json.NewDecoder(file).Decode(&record); err != nil {
		return "", importSourceInfo{}, fmt.Errorf("decode station identity import %s: %w", path, err)
	}

	normalized := callsign.Normalize(record.Callsign)
	if !callsign.IsValid(normalized) {
		slog.Warn("station identity import contains invalid callsign; using config default", "path", path, "value", record.Callsign)
		return "", importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
	}

	return normalized, importSourceInfo{path: path, mtime: stat.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}
