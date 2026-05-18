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

	box.Register("QQ0QQ-1", "QQ1ABC-1", "hello", time.Now())

	if !box.Confirm("QQ0QQ-1,RELAY-1", "QQ1ABC-1", "hello") {
		t.Fatal("Confirm returned false")
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

	if box.Confirm("QQ0QQ-1", "QQ1ABC-1", "different") {
		t.Fatal("Confirm matched different message")
	}
	if box.Confirm("QQ0QQ-1", "QQ2ABC-1", "hello") {
		t.Fatal("Confirm matched different destination")
	}
	if !box.Confirm("QQ0QQ-1", "QQ1ABC-1", "hello") {
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

			if !box.Confirm("QQ0QQ-1", "QQ1ABC-1", observed) {
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

			if box.Confirm("QQ0QQ-1", "QQ1ABC-1", observed) {
				t.Fatal("Confirm matched malformed node sequence suffix")
			}
		})
	}
}
