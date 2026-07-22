package receivelog

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

const fileDateLayout = "20060102"

type Config struct {
	Enabled       bool
	Path          string
	RetentionDays int
}

type Logger struct {
	cfg Config
	db  *sql.DB
	mu  sync.Mutex
}

type Record struct {
	ReceivedAt time.Time `json:"received_at"`
	RemoteAddr string    `json:"remote_addr"`
	Bytes      int       `json:"bytes"`
	Raw        string    `json:"raw"`
	PacketType string    `json:"packet_type,omitempty"`
	ParseError string    `json:"parse_error,omitempty"`
}

func NewSQLite(cfg Config, db *sql.DB) *Logger {
	return &Logger{cfg: cfg, db: db}
}

func (l *Logger) Append(record Record) error {
	if !l.cfg.Enabled {
		return nil
	}

	if err := l.validate(); err != nil {
		return err
	}

	return l.appendSQLite(record)
}

func (l *Logger) ReadSince(cutoff time.Time) ([]Record, error) {
	if !l.cfg.Enabled {
		return nil, nil
	}

	if err := l.validate(); err != nil {
		return nil, err
	}

	return l.readSinceSQLite(cutoff)
}

func (l *Logger) appendSQLite(record Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()

	receivedAt := record.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = now
	}
	_, err := l.db.Exec(`
		INSERT INTO receive_log(received_at, remote_addr, bytes, raw, packet_type, parse_error)
		VALUES (?, ?, ?, ?, ?, ?)
	`, receivedAt.UTC().Format(time.RFC3339Nano), record.RemoteAddr, record.Bytes, record.Raw, nullableString(record.PacketType), nullableString(record.ParseError))
	if err != nil {
		return fmt.Errorf("insert receive log: %w", err)
	}
	return nil
}

func (l *Logger) readSinceSQLite(cutoff time.Time) ([]Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rows, err := l.db.Query(`
		SELECT received_at, remote_addr, bytes, raw, packet_type, parse_error
		FROM receive_log
		WHERE received_at >= ?
		ORDER BY received_at, id
	`, cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("query receive log: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var receivedAt string
		var packetType sql.NullString
		var parseError sql.NullString
		if err := rows.Scan(&receivedAt, &record.RemoteAddr, &record.Bytes, &record.Raw, &packetType, &parseError); err != nil {
			return nil, fmt.Errorf("scan receive log: %w", err)
		}
		parsedAt, err := time.Parse(time.RFC3339Nano, receivedAt)
		if err != nil {
			return nil, fmt.Errorf("parse receive log timestamp: %w", err)
		}
		record.ReceivedAt = parsedAt
		if packetType.Valid {
			record.PacketType = packetType.String
		}
		if parseError.Valid {
			record.ParseError = parseError.String
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate receive log: %w", err)
	}
	return records, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (l *Logger) validate() error {
	if l.db == nil {
		return errors.New("receive log sqlite database is required")
	}

	if l.cfg.RetentionDays < 0 {
		return errors.New("receive log retention days must not be negative")
	}

	return nil
}
