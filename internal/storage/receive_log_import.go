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
	"time"
)

const receiveLogImportSource = "receive_log"

type receiveLogRecord struct {
	ReceivedAt time.Time `json:"received_at"`
	RemoteAddr string    `json:"remote_addr"`
	Bytes      int       `json:"bytes"`
	Raw        string    `json:"raw"`
	PacketType string    `json:"packet_type,omitempty"`
	ParseError string    `json:"parse_error,omitempty"`
}

func (db *DB) ImportReceiveLog(ctx context.Context, path string) error {
	if imported, err := db.importDone(ctx, receiveLogImportSource); err != nil {
		return err
	} else if imported {
		return nil
	}

	paths, err := receiveLogPaths(path)
	if err != nil {
		return err
	}

	var records []receiveLogRecord
	for _, sourcePath := range paths {
		fileRecords, err := readReceiveLogJSONL(sourcePath)
		if err != nil {
			return err
		}
		records = append(records, fileRecords...)
	}

	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for _, record := range records {
			if err := insertReceiveLogRecord(ctx, tx, record); err != nil {
				return err
			}
		}
		return recordImport(ctx, tx, receiveLogImportSource, importSourceInfo{path: path}, len(records))
	})
}

func receiveLogPaths(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("receive log import path is required")
	}
	if filepath.Ext(path) == ".jsonl" {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("stat receive log import %s: %w", path, err)
		}
		return []string{path}, nil
	}
	paths, err := filepath.Glob(filepath.Join(path, "received.*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("list receive log import files: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func readReceiveLogJSONL(path string) ([]receiveLogRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open receive log import %s: %w", path, err)
	}
	defer file.Close()

	var records []receiveLogRecord
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		var record receiveLogRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode receive log import %s:%d: %w", path, line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan receive log import %s: %w", path, err)
	}
	return records, nil
}

func insertReceiveLogRecord(ctx context.Context, tx *sql.Tx, record receiveLogRecord) error {
	receivedAt := record.ReceivedAt
	if receivedAt.IsZero() {
		return fmt.Errorf("receive log import record missing received_at")
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO receive_log(received_at, remote_addr, bytes, raw, packet_type, parse_error)
		VALUES (?, ?, ?, ?, ?, ?)
	`, receivedAt.UTC().Format(time.RFC3339Nano), record.RemoteAddr, record.Bytes, record.Raw, nullableImportString(record.PacketType), nullableImportString(record.ParseError))
	if err != nil {
		return fmt.Errorf("insert receive log import record: %w", err)
	}
	return nil
}

func nullableImportString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
