package httpapi

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteSessionStoreLoadsValidSessionAfterRestart(t *testing.T) {
	db := openSessionTestDB(t)
	firstStore := newSQLiteSessionStore(db)
	token, _, err := firstStore.create(time.Hour)
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}

	restartedStore := newSQLiteSessionStore(db)
	if !restartedStore.valid(token) {
		t.Fatal("session not valid after sqlite store restart")
	}
}

func TestSQLiteSessionStoreDeletePersistsAcrossRestart(t *testing.T) {
	db := openSessionTestDB(t)
	store := newSQLiteSessionStore(db)
	token, _, err := store.create(time.Hour)
	if err != nil {
		t.Fatalf("create() error = %v", err)
	}
	if err := store.delete(token); err != nil {
		t.Fatalf("delete() error = %v", err)
	}
	if newSQLiteSessionStore(db).valid(token) {
		t.Fatal("deleted session valid after sqlite restart")
	}
}

func TestSQLiteSessionStoreDoesNotLoadExpiredSession(t *testing.T) {
	db := openSessionTestDB(t)
	if _, err := db.ExecContext(context.Background(), `INSERT INTO http_sessions(token_hash, expires_at) VALUES ('expired', ?)`, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}
	store := newSQLiteSessionStore(db)
	if len(store.sessions) != 0 {
		t.Fatalf("loaded sessions = %d, want 0", len(store.sessions))
	}
}

func openSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE http_sessions (token_hash TEXT PRIMARY KEY, expires_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create http_sessions: %v", err)
	}
	return db
}
