package chatstatus

import (
	"testing"
	"time"
)

func TestMarkReadStoresReadMarker(t *testing.T) {
	db := openChatStatusTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	readAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	s.MarkRead("P_broadcast", readAt)
	if err := s.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty() error = %v", err)
	}

	loaded, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite() reload error = %v", err)
	}
	reads := loaded.SnapshotReads()
	if !reads["P_broadcast"].Equal(readAt) {
		t.Fatalf("last read = %v, want %v", reads["P_broadcast"], readAt)
	}
}

func TestRecordIncomingNoopForSQLiteDerivedSnapshots(t *testing.T) {
	db := openChatStatusTestDB(t)
	s, err := NewSQLite(db)
	if err != nil {
		t.Fatalf("NewSQLite() error = %v", err)
	}
	s.RecordIncoming("P_broadcast", time.Now(), "hello")
	if len(s.SnapshotReads()) != 0 {
		t.Fatal("RecordIncoming should not create read markers")
	}
}
