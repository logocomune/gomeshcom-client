package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestImportHTTPSessionsImportsValidSessions(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	expiresAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	expiredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	writeFile(t, path, fmt.Sprintf(`{"sessions":{"valid-token":%q,"expired-token":%q}}`, expiresAt, expiredAt))

	if err := db.ImportHTTPSessions(ctx, path); err != nil {
		t.Fatalf("ImportHTTPSessions() error = %v", err)
	}

	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT expires_at FROM http_sessions WHERE token_hash = 'valid-token'`).Scan(&got); err != nil {
		t.Fatalf("query valid session: %v", err)
	}
	if got != expiresAt {
		t.Fatalf("expires_at = %q, want %q", got, expiresAt)
	}
	var expiredCount int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM http_sessions WHERE token_hash = 'expired-token'`).Scan(&expiredCount); err != nil {
		t.Fatalf("query expired session: %v", err)
	}
	if expiredCount != 0 {
		t.Fatalf("expired sessions = %d, want 0", expiredCount)
	}
	assertImportRecorded(t, db.SQL(), httpSessionsImportSource, 1)
}

func TestImportHTTPSessionsMissingFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	if err := db.ImportHTTPSessions(ctx, filepath.Join(t.TempDir(), "http-sessions.json")); err != nil {
		t.Fatalf("ImportHTTPSessions() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), httpSessionsImportSource, 0)
}

func TestImportHTTPSessionsMalformedFileRecordsEmptyImport(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	writeFile(t, path, "not-json")
	if err := db.ImportHTTPSessions(ctx, path); err != nil {
		t.Fatalf("ImportHTTPSessions() error = %v", err)
	}
	assertImportRecorded(t, db.SQL(), httpSessionsImportSource, 0)
}

func TestImportHTTPSessionsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	path := filepath.Join(t.TempDir(), "http-sessions.json")
	first := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	second := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339Nano)
	writeFile(t, path, fmt.Sprintf(`{"sessions":{"token":%q}}`, first))
	if err := db.ImportHTTPSessions(ctx, path); err != nil {
		t.Fatalf("first ImportHTTPSessions() error = %v", err)
	}
	writeFile(t, path, fmt.Sprintf(`{"sessions":{"token":%q}}`, second))
	if err := db.ImportHTTPSessions(ctx, path); err != nil {
		t.Fatalf("second ImportHTTPSessions() error = %v", err)
	}
	var got string
	if err := db.SQL().QueryRowContext(ctx, `SELECT expires_at FROM http_sessions WHERE token_hash = 'token'`).Scan(&got); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if got != first {
		t.Fatalf("expires_at = %q, want first import %q", got, first)
	}
}
