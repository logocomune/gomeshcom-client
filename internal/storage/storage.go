package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	schemaVersion     = 3
	sqliteBusyTimeout = 5 * time.Second
)

type DB struct {
	conn *sql.DB
}

type PurgePolicy struct {
	Interval            time.Duration
	ReceiveLogRetention time.Duration
	PublicChatRetention time.Duration
	NodesRetention      time.Duration
	TelemetryRetention  time.Duration
}

func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	db := &DB{conn: conn}
	if err := db.configure(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := db.migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) SQL() *sql.DB {
	return db.conn
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func ensureChatColumns(ctx context.Context, tx *sql.Tx, table string) error {
	columns, err := tableColumns(ctx, tx, table)
	if err != nil {
		return err
	}
	missingColumns := map[string]string{
		"via":             "ALTER TABLE " + table + " ADD COLUMN via TEXT CHECK (via IS NULL OR (json_valid(via) AND json_type(via) = 'array'))",
		"sequence_id":     "ALTER TABLE " + table + " ADD COLUMN sequence_id TEXT",
		"ack_status":      "ALTER TABLE " + table + " ADD COLUMN ack_status TEXT CHECK(ack_status IS NULL OR ack_status IN ('ack', 'reject'))",
		"ack_received_at": "ALTER TABLE " + table + " ADD COLUMN ack_received_at TEXT",
		"ack_src":         "ALTER TABLE " + table + " ADD COLUMN ack_src TEXT",
		"ack_src_type":    "ALTER TABLE " + table + " ADD COLUMN ack_src_type TEXT",
		"ack_rssi":        "ALTER TABLE " + table + " ADD COLUMN ack_rssi INTEGER",
		"ack_snr":         "ALTER TABLE " + table + " ADD COLUMN ack_snr INTEGER",
		"ack_via":         "ALTER TABLE " + table + " ADD COLUMN ack_via TEXT CHECK (ack_via IS NULL OR (json_valid(ack_via) AND json_type(ack_via) = 'array'))",
	}
	order := []string{"via"}
	if table == "chats_dm" {
		order = append(order, "sequence_id", "ack_status", "ack_received_at", "ack_src", "ack_src_type", "ack_rssi", "ack_snr", "ack_via")
	}
	for _, column := range order {
		if columns[column] {
			continue
		}
		if _, err := tx.ExecContext(ctx, missingColumns[column]); err != nil {
			return fmt.Errorf("add sqlite %s.%s: %w", table, column, err)
		}
	}
	return nil
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, fmt.Errorf("query sqlite table info %s: %w", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan sqlite table info %s: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite table info %s: %w", table, err)
	}
	return columns, nil
}

func (db *DB) Health(ctx context.Context) error {
	if err := db.conn.PingContext(ctx); err != nil {
		return fmt.Errorf("sqlite ping: %w", err)
	}
	return nil
}

func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback sqlite transaction after %w: %w", err, rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite transaction: %w", err)
	}
	return nil
}

func (db *DB) StartPurge(ctx context.Context, policy PurgePolicy) {
	if policy.Interval <= 0 {
		return
	}
	if err := db.Purge(ctx, policy, time.Now().UTC()); err != nil {
		slog.Error("sqlite purge failed", "error", err)
	}
	ticker := time.NewTicker(policy.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := db.Purge(ctx, policy, now.UTC()); err != nil {
				slog.Error("sqlite purge failed", "error", err)
			}
		}
	}
}

func (db *DB) Purge(ctx context.Context, policy PurgePolicy, now time.Time) error {
	if err := db.WithTx(ctx, func(tx *sql.Tx) error {
		if err := purgeBefore(ctx, tx, "receive_log", "received_at", policy.ReceiveLogRetention, now); err != nil {
			return err
		}
		if err := purgeBefore(ctx, tx, "chats_public", "received_at", policy.PublicChatRetention, now); err != nil {
			return err
		}
		if err := purgeBefore(ctx, tx, "nodes", "lastseen", policy.NodesRetention, now); err != nil {
			return err
		}
		if err := purgeBefore(ctx, tx, "telemetry_samples", "received_at", policy.TelemetryRetention, now); err != nil {
			return err
		}
		if err := purgeBefore(ctx, tx, "telemetry_direct_signal", "received_at", policy.TelemetryRetention, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return db.truncateWAL(ctx)
}

func purgeBefore(ctx context.Context, tx *sql.Tx, table string, column string, retention time.Duration, now time.Time) error {
	if retention <= 0 {
		return nil
	}
	cutoff := now.Add(-retention).UTC().Format(time.RFC3339Nano)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s IS NOT NULL AND %s < ?", table, column, column)
	if _, err := tx.ExecContext(ctx, query, cutoff); err != nil {
		return fmt.Errorf("purge sqlite %s: %w", table, err)
	}
	return nil
}

func (db *DB) truncateWAL(ctx context.Context) error {
	if _, err := db.conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("truncate sqlite wal: %w", err)
	}
	return nil
}

func (db *DB) configure(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeout.Milliseconds()),
	}
	for _, pragma := range pragmas {
		if _, err := db.conn.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure sqlite %q: %w", pragma, err)
		}
	}
	return nil
}

func (db *DB) migrate(ctx context.Context) error {
	return db.WithTx(ctx, func(tx *sql.Tx) error {
		for _, stmt := range schemaStatements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply sqlite schema: %w", err)
			}
			if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS chats_dm") {
				if err := ensureChatColumns(ctx, tx, "chats_dm"); err != nil {
					return err
				}
			}
			if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS chats_public") {
				if err := ensureChatColumns(ctx, tx, "chats_public"); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, schemaVersion); err != nil {
			return fmt.Errorf("record sqlite schema migration: %w", err)
		}
		return nil
	})
}
