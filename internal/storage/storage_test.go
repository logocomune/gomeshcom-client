package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMigratesSchema(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	assertTableExists(t, db.SQL(), "receive_log")
	assertTableExists(t, db.SQL(), "telemetry_samples")
	assertTableExists(t, db.SQL(), "telemetry_direct_signal")
	assertTableExists(t, db.SQL(), "chats_dm")
	assertTableExists(t, db.SQL(), "http_sessions")
	assertMigrationVersion(t, db.SQL(), schemaVersion)

	if err := db.Health(ctx); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}

func TestOpenMigrationIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gomeshcom.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	assertMigrationVersion(t, db.SQL(), schemaVersion)
}

func TestOpenConfiguresSQLiteContentionHandling(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	if maxOpen := db.SQL().Stats().MaxOpenConnections; maxOpen != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", maxOpen)
	}

	var busyTimeout int
	if err := db.SQL().QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if int64(busyTimeout) != sqliteBusyTimeout.Milliseconds() {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeout.Milliseconds())
	}
}

func TestOpenAddsAckColumnsToExistingChatsDM(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gomeshcom.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = legacy.ExecContext(ctx, `
		CREATE TABLE chats_dm (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			msg_id TEXT,
			received_at TEXT NOT NULL,
			src TEXT,
			src_type TEXT,
			dst TEXT NOT NULL,
			msg TEXT NOT NULL,
			rssi INTEGER,
			snr INTEGER,
			direction TEXT,
			delivery_status TEXT
		)
	`)
	if err != nil {
		t.Fatalf("create legacy chats_dm: %v", err)
	}
	_, err = legacy.ExecContext(ctx, `
		CREATE TABLE chats_public (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			channel TEXT,
			msg_id TEXT,
			received_at TEXT NOT NULL,
			src TEXT,
			src_type TEXT,
			dst TEXT NOT NULL,
			msg TEXT NOT NULL,
			rssi INTEGER,
			snr INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("create legacy chats_public: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, column := range []string{"via", "sequence_id", "ack_status", "ack_received_at", "ack_src", "ack_src_type", "ack_rssi", "ack_snr", "ack_via"} {
		assertColumnExists(t, db.SQL(), "chats_dm", column)
	}
	assertColumnExists(t, db.SQL(), "chats_public", "via")
}

func TestNodeViaRequiresJSONArray(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	_, err := db.SQL().ExecContext(ctx, `INSERT INTO nodes(node_id, via) VALUES ('OK1', '["A","B"]')`)
	if err != nil {
		t.Fatalf("insert array via error = %v", err)
	}

	_, err = db.SQL().ExecContext(ctx, `INSERT INTO nodes(node_id, via) VALUES ('BAD1', '{"via":"A"}')`)
	if err == nil {
		t.Fatal("expected object via insert to fail")
	}
}

func TestStatsChannelsCascadeWithForeignKeys(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)

	_, err := db.SQL().ExecContext(ctx, `INSERT INTO stats_hourly(hour_unix, dm, dm_ack, public, telemetry, position, errors, total) VALUES (1, 1, 0, 2, 3, 4, 0, 10)`)
	if err != nil {
		t.Fatalf("insert stats_hourly error = %v", err)
	}
	_, err = db.SQL().ExecContext(ctx, `INSERT INTO stats_channels(hour_unix, kind, target, count) VALUES (1, 'broadcast', '*', 2)`)
	if err != nil {
		t.Fatalf("insert stats_channels error = %v", err)
	}
	_, err = db.SQL().ExecContext(ctx, `DELETE FROM stats_hourly WHERE hour_unix = 1`)
	if err != nil {
		t.Fatalf("delete stats_hourly error = %v", err)
	}

	var count int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM stats_channels WHERE hour_unix = 1`).Scan(&count); err != nil {
		t.Fatalf("count stats_channels error = %v", err)
	}
	if count != 0 {
		t.Fatalf("stats_channels count = %d, want 0", count)
	}
}

func TestPurgeDeletesExpiredReceiveLogPublicChatsAndNodes(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	old := now.Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := now.Add(-2 * 24 * time.Hour).Format(time.RFC3339Nano)

	execSQL(t, db.SQL(), `INSERT INTO receive_log(received_at, remote_addr, bytes, raw) VALUES (?, '127.0.0.1:1799', 1, '{}')`, old)
	execSQL(t, db.SQL(), `INSERT INTO receive_log(received_at, remote_addr, bytes, raw) VALUES (?, '127.0.0.1:1799', 1, '{}')`, recent)
	execSQL(t, db.SQL(), `INSERT INTO chats_public(conversation_id, kind, received_at, dst, msg) VALUES ('P_broadcast', 'broadcast', ?, '*', 'old')`, old)
	execSQL(t, db.SQL(), `INSERT INTO chats_public(conversation_id, kind, received_at, dst, msg) VALUES ('P_broadcast', 'broadcast', ?, '*', 'recent')`, recent)
	execSQL(t, db.SQL(), `INSERT INTO nodes(node_id, lastseen) VALUES ('OLD', ?)`, old)
	execSQL(t, db.SQL(), `INSERT INTO nodes(node_id, lastseen) VALUES ('RECENT', ?)`, recent)
	execSQL(t, db.SQL(), `INSERT INTO telemetry_samples(received_at, src, src_origin, metric, value) VALUES (?, 'QQ1OLD-1', 'QQ1OLD-1', 'temp1', 1)`, old)
	execSQL(t, db.SQL(), `INSERT INTO telemetry_samples(received_at, src, src_origin, metric, value) VALUES (?, 'QQ1NEW-1', 'QQ1NEW-1', 'temp1', 2)`, recent)
	execSQL(t, db.SQL(), `INSERT INTO telemetry_direct_signal(received_at, src, src_origin, rssi) VALUES (?, 'QQ1OLD-1', 'QQ1OLD-1', -100)`, old)
	execSQL(t, db.SQL(), `INSERT INTO telemetry_direct_signal(received_at, src, src_origin, rssi) VALUES (?, 'QQ1NEW-1', 'QQ1NEW-1', -90)`, recent)

	err := db.Purge(ctx, PurgePolicy{
		ReceiveLogRetention: 7 * 24 * time.Hour,
		PublicChatRetention: 7 * 24 * time.Hour,
		NodesRetention:      7 * 24 * time.Hour,
		TelemetryRetention:  7 * 24 * time.Hour,
	}, now)
	if err != nil {
		t.Fatalf("Purge() error = %v", err)
	}

	assertTableCount(t, db.SQL(), "receive_log", 1)
	assertTableCount(t, db.SQL(), "chats_public", 1)
	assertTableCount(t, db.SQL(), "nodes", 1)
	assertTableCount(t, db.SQL(), "telemetry_samples", 1)
	assertTableCount(t, db.SQL(), "telemetry_direct_signal", 1)
	assertRowExists(t, db.SQL(), "nodes", "node_id", "RECENT")
}

func TestWithTxRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	failure := errors.New("fail")

	err := db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO dm_stats(callsign, sent, ack) VALUES ('IU5PMP-1', 1, 0)`)
		if err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("WithTx() error = %v, want %v", err, failure)
	}

	var count int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM dm_stats`).Scan(&count); err != nil {
		t.Fatalf("count dm_stats error = %v", err)
	}
	if count != 0 {
		t.Fatalf("dm_stats count = %d, want 0", count)
	}
}

func openTestDB(t *testing.T, ctx context.Context) *DB {
	t.Helper()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func execSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q error = %v", query, err)
	}
}

func assertTableCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		t.Fatalf("count %s error = %v", table, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}

func assertRowExists(t *testing.T, db *sql.DB, table, column, value string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`, value).Scan(&count); err != nil {
		t.Fatalf("exists %s.%s error = %v", table, column, err)
	}
	if count != 1 {
		t.Fatalf("%s.%s=%s count = %d, want 1", table, column, value, count)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err != nil {
		t.Fatalf("table %s missing: %v", table, err)
	}
}

func assertMigrationVersion(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&count)
	if err != nil {
		t.Fatalf("query schema_migrations error = %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations version %d count = %d, want 1", version, count)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table info %s error = %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info %s error = %v", table, err)
		}
		if name == column {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info %s error = %v", table, err)
	}
	t.Fatalf("column %s.%s missing", table, column)
}
