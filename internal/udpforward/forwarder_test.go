package udpforward

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// udpSink starts a local UDP listener and returns its address and a channel
// that receives each datagram payload.
func udpSink(t testing.TB) (addr string, payloads <-chan []byte) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("udpSink ListenUDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ch := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 65535)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			ch <- pkt
		}
	}()
	return conn.LocalAddr().String(), ch
}

func recvWithTimeout(t *testing.T, ch <-chan []byte, d time.Duration) []byte {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(d):
		t.Fatalf("timeout waiting for forwarded datagram")
		return nil
	}
}

func TestForwardZeroTargets(t *testing.T) {
	f, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	defer f.Close()
	// Must be a no-op without panic.
	f.Forward([]byte("hello"))
}

func TestForwardToSingleSink(t *testing.T) {
	addr, payloads := udpSink(t)
	f, err := New([]string{addr})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer f.Close()

	want := []byte("test-payload-123")
	f.Forward(want)

	got := recvWithTimeout(t, payloads, time.Second)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestForwardToMultipleSinks(t *testing.T) {
	addr1, ch1 := udpSink(t)
	addr2, ch2 := udpSink(t)
	addr3, ch3 := udpSink(t)

	f, err := New([]string{addr1, addr2, addr3})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer f.Close()

	want := []byte("fan-out-payload")
	f.Forward(want)

	for _, ch := range []<-chan []byte{ch1, ch2, ch3} {
		got := recvWithTimeout(t, ch, time.Second)
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestForwardDoesNotMutateOriginalBuffer(t *testing.T) {
	addr, payloads := udpSink(t)
	f, err := New([]string{addr})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer f.Close()

	original := []byte("original")
	sent := make([]byte, len(original))
	copy(sent, original)

	f.Forward(original)
	// Mutate original after Forward returns.
	for i := range original {
		original[i] = 'X'
	}

	got := recvWithTimeout(t, payloads, time.Second)
	if !bytes.Equal(got, sent) {
		t.Fatalf("buffer mutation affected forwarded data: got %q, want %q", got, sent)
	}
}

func TestForwardDropsOnFullChannel(t *testing.T) {
	// Fill the channel past capacity without a reader. Must not block or panic.
	addr, _ := udpSink(t)
	f, err := New([]string{addr})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer f.Close()

	// Pause the worker goroutine's reads by holding done closed temporarily — instead,
	// just flood more than the channel capacity synchronously; drop-oldest must handle it.
	payload := bytes.Repeat([]byte("x"), 100)
	for i := 0; i < workerChannelCap*3; i++ {
		f.Forward(payload) // must never block
	}
}

func TestForwardDataIntegrity(t *testing.T) {
	payloads := [][]byte{
		{},
		{0x00},
		[]byte("hello world"),
		bytes.Repeat([]byte{0xff}, 1400),
		[]byte(`{"type":"msg","src":"QQ1ABC-1","dst":"*","msg":"test"}`),
	}

	for _, want := range payloads {
		t.Run("", func(t *testing.T) {
			addr, ch := udpSink(t)
			f, err := New([]string{addr})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer f.Close()

			f.Forward(want)

			got := recvWithTimeout(t, ch, time.Second)
			if !bytes.Equal(got, want) {
				t.Fatalf("integrity failure: got %d bytes, want %d bytes", len(got), len(want))
			}
		})
	}
}

func FuzzForwardNoPanic(f *testing.F) {
	seeds := [][]byte{
		{},
		[]byte("hello"),
		[]byte(`{"type":"msg"}`),
		bytes.Repeat([]byte{0xff}, 1400),
		{0x00, 0x01, 0x02},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	addr, payloads := udpSink(f)
	fwd, err := New([]string{addr})
	if err != nil {
		f.Fatalf("New: %v", err)
	}
	f.Cleanup(func() { fwd.Close() })

	f.Fuzz(func(t *testing.T, data []byte) {
		original := make([]byte, len(data))
		copy(original, data)

		fwd.Forward(data)

		// Drain any received packet — we only care about no-panic and no mutation.
		select {
		case got := <-payloads:
			// The received payload must equal the original data (before any potential mutation).
			if !bytes.Equal(got, original) {
				t.Fatalf("data integrity: got %x, want %x", got, original)
			}
		case <-time.After(50 * time.Millisecond):
			// Acceptable: channel might be full; drop-oldest is expected behaviour.
		}
	})
}
