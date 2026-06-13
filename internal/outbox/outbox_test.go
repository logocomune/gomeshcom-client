package outbox

import (
	"testing"
	"time"
)

func TestOutboxExpiresUnconfirmedMessage(t *testing.T) {
	failed := make(chan PendingMessage, 1)
	box := New(10*time.Millisecond, func(message PendingMessage) {
		failed <- message
	})

	createdAt := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", createdAt)

	select {
	case message := <-failed:
		if message.Source != "QQ0QQ-1" {
			t.Fatalf("Source = %q, want QQ0QQ-1", message.Source)
		}
		if message.Destination != "QQ1ABC-1" {
			t.Fatalf("Destination = %q, want QQ1ABC-1", message.Destination)
		}
		if message.Message != "hello" {
			t.Fatalf("Message = %q, want hello", message.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("pending message did not expire")
	}
}

func TestOutboxConfirmSuppressesFailure(t *testing.T) {
	failed := make(chan PendingMessage, 1)
	box := New(20*time.Millisecond, func(message PendingMessage) {
		failed <- message
	})

	createdAt := time.Now()
	box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", createdAt)

	pending, ok := box.Confirm("QQ0QQ-1,RELAY-1", "QQ1ABC-1", "hello")
	if !ok {
		t.Fatal("Confirm returned false")
	}
	if pending.Destination != "QQ1ABC-1" {
		t.Errorf("returned pending has wrong destination: %q", pending.Destination)
	}
	if pending.CreatedAt.IsZero() {
		t.Error("returned pending has zero CreatedAt")
	}

	select {
	case message := <-failed:
		t.Fatalf("confirmed message expired: %+v", message)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestOutboxConfirmRequiresDestinationAndMessage(t *testing.T) {
	box := New(time.Minute, nil)
	box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", time.Now())

	if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", "different"); ok {
		t.Fatal("Confirm matched different message")
	}
	if _, ok := box.Confirm("QQ0QQ-1", "QQ2ABC-1", "hello"); ok {
		t.Fatal("Confirm matched different destination")
	}
	if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", "hello"); !ok {
		t.Fatal("Confirm did not match original message")
	}
}

func TestOutboxConfirmAcceptsNodeSequenceSuffix(t *testing.T) {
	tests := map[string]string{
		"complete suffix":  "hello{571}",
		"truncated suffix": "hello{571",
	}

	for name, observed := range tests {
		t.Run(name, func(t *testing.T) {
			failed := make(chan PendingMessage, 1)
			box := New(20*time.Millisecond, func(message PendingMessage) {
				failed <- message
			})

			box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", time.Now())

			if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", observed); !ok {
				t.Fatal("Confirm did not match node sequence suffix")
			}

			select {
			case message := <-failed:
				t.Fatalf("confirmed message expired: %+v", message)
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}

func TestOutboxConfirmRejectsMalformedNodeSequenceSuffix(t *testing.T) {
	tests := map[string]string{
		"empty suffix":     "hello{}",
		"empty truncated":  "hello{",
		"non digit suffix": "hello{abc",
	}

	for name, observed := range tests {
		t.Run(name, func(t *testing.T) {
			box := New(time.Minute, nil)
			box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", time.Now())

			if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", observed); ok {
				t.Fatal("Confirm matched malformed node sequence suffix")
			}
		})
	}
}

// TestOutboxConfirmReturnsPendingCreatedAt verifies that the returned
// PendingMessage preserves the original CreatedAt for latency calculation.
func TestOutboxConfirmReturnsPendingCreatedAt(t *testing.T) {
	box := New(time.Minute, nil)
	createdAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	box.Register("QQ0QQ-1", "QQ1ABC-1", "ping", createdAt)

	pending, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", "ping")
	if !ok {
		t.Fatal("Confirm returned false")
	}
	if !pending.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt: want %v, got %v", createdAt, pending.CreatedAt)
	}
}

// TestOutboxConfirmDedup verifies that a second Confirm for the same message
// returns false (first ack wins).
func TestOutboxConfirmDedup(t *testing.T) {
	box := New(time.Minute, nil)
	box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", time.Now())

	if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", "hello"); !ok {
		t.Fatal("first Confirm returned false")
	}
	if _, ok := box.Confirm("QQ0QQ-1", "QQ1ABC-1", "hello"); ok {
		t.Fatal("second Confirm should return false (dedup)")
	}
}
