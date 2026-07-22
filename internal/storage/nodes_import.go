package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/positions"
)

const nodesImportSource = "nodes"

func (db *DB) ImportNodes(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, nodesImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	records, sourceInfo, err := readNodeRecords(path)
	if err != nil {
		return err
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for nodeID, record := range records {
			if err := insertNode(ctx, tx, nodeID, record); err != nil {
				return err
			}
		}
		return recordImport(ctx, tx, nodesImportSource, sourceInfo, len(records))
	})
}

func (db *DB) importDone(ctx context.Context, source string) (bool, error) {
	var exists int
	err := db.conn.QueryRowContext(ctx, `SELECT 1 FROM data_imports WHERE source = ?`, source).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, fmt.Errorf("check import %s: %w", source, err)
}

type importSourceInfo struct {
	path  string
	mtime string
}

func readNodeRecords(path string) (map[string]positions.Record, importSourceInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]positions.Record{}, importSourceInfo{path: path}, nil
		}
		return nil, importSourceInfo{}, fmt.Errorf("open nodes import %s: %w", path, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("stat nodes import %s: %w", path, err)
	}

	var records map[string]positions.Record
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		return nil, importSourceInfo{}, fmt.Errorf("decode nodes import %s: %w", path, err)
	}
	if records == nil {
		records = map[string]positions.Record{}
	}

	return records, importSourceInfo{
		path:  path,
		mtime: stat.ModTime().UTC().Format(time.RFC3339Nano),
	}, nil
}

func insertNode(ctx context.Context, tx *sql.Tx, nodeID string, record positions.Record) error {
	via, err := json.Marshal(normalizeVia(record.Via))
	if err != nil {
		return fmt.Errorf("encode node %s via: %w", nodeID, err)
	}

	var lastDirectSeen any
	if record.LastDirectSeen != nil {
		lastDirectSeen = record.LastDirectSeen.UTC().Format(time.RFC3339Nano)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO nodes(
			node_id, lat, lng, alt, hw_id, firstseen, lastseen, lastdirectseen, rssi, snr, via
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			lat = excluded.lat,
			lng = excluded.lng,
			alt = excluded.alt,
			hw_id = excluded.hw_id,
			firstseen = excluded.firstseen,
			lastseen = excluded.lastseen,
			lastdirectseen = excluded.lastdirectseen,
			rssi = excluded.rssi,
			snr = excluded.snr,
			via = excluded.via
	`,
		nodeID,
		record.Latitude,
		record.Longitude,
		record.Altitude,
		record.HardwareID,
		record.FirstSeen.UTC().Format(time.RFC3339Nano),
		record.LastSeen.UTC().Format(time.RFC3339Nano),
		lastDirectSeen,
		record.RSSI,
		record.SNR,
		string(via),
	)
	if err != nil {
		return fmt.Errorf("insert node %s: %w", nodeID, err)
	}
	return nil
}

func normalizeVia(via []string) []string {
	if via == nil {
		return []string{}
	}
	return via
}

func recordImport(ctx context.Context, tx *sql.Tx, source string, info importSourceInfo, count int) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO data_imports(source, imported_at, source_path, source_mtime, record_count)
		VALUES (?, datetime('now'), ?, ?, ?)
	`, source, info.path, info.mtime, count)
	if err != nil {
		return fmt.Errorf("record import %s: %w", source, err)
	}
	return nil
}
