package udpforward

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
)

const workerChannelCap = 256

type worker struct {
	conn net.Conn
	ch   chan []byte
}

// Forwarder mirrors received UDP datagrams unmodified to one or more downstream targets.
// Each target runs an independent goroutine so a slow target never backpressures others.
type Forwarder struct {
	workers []worker
	done    chan struct{}
	wg      sync.WaitGroup
}

// New dials each target and starts a forwarding goroutine per target.
// targets must be valid host:port strings (use config.ParseForwardTargets to validate).
func New(targets []string) (*Forwarder, error) {
	f := &Forwarder{done: make(chan struct{})}
	for _, target := range targets {
		conn, err := net.Dial("udp", target)
		if err != nil {
			f.closeWorkers()
			return nil, fmt.Errorf("dial udp forward target %q: %w", target, err)
		}
		w := worker{conn: conn, ch: make(chan []byte, workerChannelCap)}
		f.workers = append(f.workers, w)
		f.wg.Add(1)
		go f.runWorker(w)
	}
	return f, nil
}

func (f *Forwarder) runWorker(w worker) {
	defer f.wg.Done()
	for {
		select {
		case <-f.done:
			return
		case data := <-w.ch:
			if _, err := w.conn.Write(data); err != nil {
				slog.Error("udp forward write failed", "addr", w.conn.RemoteAddr(), "error", err)
			}
		}
	}
}

// Forward sends a copy of data to all target channels without blocking.
// If a target channel is full the oldest item is dropped to make room (best-effort UDP semantics).
func (f *Forwarder) Forward(data []byte) {
	if len(f.workers) == 0 {
		return
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	for i := range f.workers {
		w := &f.workers[i]
		select {
		case w.ch <- cp:
		default:
			// drop oldest, enqueue newest
			select {
			case <-w.ch:
			default:
			}
			select {
			case w.ch <- cp:
			default:
			}
		}
	}
}

// Close signals all goroutines to stop and closes all connections.
func (f *Forwarder) Close() error {
	close(f.done)
	f.wg.Wait()
	return f.closeWorkers()
}

func (f *Forwarder) closeWorkers() error {
	var firstErr error
	for _, w := range f.workers {
		if err := w.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
