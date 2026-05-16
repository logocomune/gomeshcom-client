package events

import (
	"context"
	"testing"
	"time"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriber := bus.Subscribe(ctx)

	want := Event{Type: "test", Data: "hello"}
	bus.Publish(want)

	select {
	case got := <-subscriber:
		if got.Type != want.Type {
			t.Errorf("got type %q, want %q", got.Type, want.Type)
		}
		if got.Data != want.Data {
			t.Errorf("got data %v, want %v", got.Data, want.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBusUnsubscribeOnContextDone(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())

	subscriber := bus.Subscribe(ctx)
	
	cancel()
	
	// Wait for cleanup goroutine to run
	time.Sleep(10 * time.Millisecond)

	bus.mu.Lock()
	count := len(bus.subscribers)
	bus.mu.Unlock()

	if count != 0 {
		t.Errorf("subscriber count = %d, want 0", count)
	}

	// Channel should be closed
	select {
	case _, ok := <-subscriber:
		if ok {
			t.Error("channel not closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestBusNonBlockingPublish(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe but don't read
	bus.Subscribe(ctx)

	// Publish multiple events. Since buffer is 16, this should not block
	// even if we don't read, but the code uses a non-blocking select for publish
	// so it should NEVER block.
	for i := 0; i < 20; i++ {
		bus.Publish(Event{Type: "test", Data: i})
	}
}
