package events

import (
	"context"
	"sync"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Bus struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func NewBus() *Bus {
	return &Bus{subscribers: make(map[chan Event]struct{})}
}

func (b *Bus) Publish(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe(ctx context.Context) <-chan Event {
	subscriber := make(chan Event, 16)

	b.mu.Lock()
	b.subscribers[subscriber] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, subscriber)
		close(subscriber)
		b.mu.Unlock()
	}()

	return subscriber
}
