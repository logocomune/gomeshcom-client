package serialbridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/packetingest"
)

type portRead struct {
	data []byte
	err  error
}

type scriptedPort struct {
	reads chan portRead

	closeOnce  sync.Once
	closed     chan struct{}
	closeCount atomic.Int32

	writeMu     sync.Mutex
	writes      [][]byte
	writeLimit  int
	writeZero   bool
	writeCalls  int
	failWriteAt int
	writeError  error
}

func newScriptedPort() *scriptedPort {
	return &scriptedPort{
		reads:  make(chan portRead, 16),
		closed: make(chan struct{}),
	}
}

func (p *scriptedPort) Read(buffer []byte) (int, error) {
	select {
	case result := <-p.reads:
		count := copy(buffer, result.data)
		return count, result.err
	case <-p.closed:
		return 0, io.ErrClosedPipe
	}
}

func (p *scriptedPort) Write(data []byte) (int, error) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	p.writeCalls++
	if p.writeZero {
		return 0, nil
	}
	if p.failWriteAt > 0 && p.writeCalls >= p.failWriteAt {
		if p.writeError == nil {
			return 0, io.ErrClosedPipe
		}
		return 0, p.writeError
	}
	count := len(data)
	if p.writeLimit > 0 && count > p.writeLimit {
		count = p.writeLimit
	}
	p.writes = append(p.writes, append([]byte(nil), data[:count]...))
	return count, nil
}

func (p *scriptedPort) Close() error {
	p.closeCount.Add(1)
	p.closeOnce.Do(func() {
		close(p.closed)
	})
	return nil
}

func (p *scriptedPort) writtenBytes() []byte {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return bytes.Join(p.writes, nil)
}

type openResult struct {
	port Port
	err  error
}

type scriptedOpener struct {
	results chan openResult
	mu      sync.Mutex
	configs []PortConfig
}

func newScriptedOpener(results ...openResult) *scriptedOpener {
	opener := &scriptedOpener{results: make(chan openResult, len(results)+4)}
	for _, result := range results {
		opener.results <- result
	}
	return opener
}

func (o *scriptedOpener) Open(config PortConfig) (Port, error) {
	o.mu.Lock()
	o.configs = append(o.configs, config)
	o.mu.Unlock()
	result := <-o.results
	return result.port, result.err
}

func (o *scriptedOpener) configurations() []PortConfig {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]PortConfig(nil), o.configs...)
}

type recordingForwarder struct {
	mu       sync.Mutex
	payloads [][]byte
}

type panicForwarder struct {
	called bool
}

func (f *panicForwarder) Forward([]byte) {
	f.called = true
}

func (f *recordingForwarder) Forward(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payloads = append(f.payloads, append([]byte(nil), data...))
}

func (f *recordingForwarder) snapshot() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.payloads...)
}

func TestBridgeRunProcessesTrafficSendsAndStops(t *testing.T) {
	port := newScriptedPort()
	opener := newScriptedOpener(openResult{port: port})
	bus := events.NewBus()
	eventContext, cancelEvents := context.WithCancel(context.Background())
	defer cancelEvents()
	subscriber := bus.Subscribe(eventContext)
	forwarder := &recordingForwarder{}
	bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{Bus: bus}), forwarder)

	runContext, cancelRun := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() {
		runResult <- bridge.Run(runContext)
	}()
	waitForBridgeState(t, bridge, StateConnected)

	if got := string(port.writtenBytes()); got != enablePacketOutputCommand {
		t.Fatalf("startup write = %q, want %q", got, enablePacketOutputCommand)
	}
	configs := opener.configurations()
	if len(configs) != 1 ||
		configs[0].Device != "/dev/ttyUSB0" ||
		configs[0].Baud != 115200 ||
		configs[0].DTR ||
		configs[0].RTS {
		t.Fatalf("open configurations = %+v", configs)
	}

	if err := bridge.SendText(context.Background(), "qq1peer-2", "Hello", 149); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	wantWrites := enablePacketOutputCommand + "::{QQ1PEER-2}Hello\r\n"
	if got := string(port.writtenBytes()); got != wantWrites {
		t.Fatalf("writes = %q, want %q", got, wantWrites)
	}

	payload := `{"src_type":"node","type":"msg","src":"QQ1OWN-1","dst":"QQ1PEER-2","msg":"Hello{663"}`
	port.reads <- portRead{data: []byte("[EXT] Out: " + payload + " Len: 100\r\n")}
	event := readSerialEvent(t, subscriber)
	if event.Type != "packet.received" {
		t.Fatalf("event type = %q, want packet.received", event.Type)
	}
	eventPayload := event.Data.(map[string]any)
	if eventPayload["transport"] != "serial" || eventPayload["endpoint"] != "/dev/ttyUSB0" {
		t.Fatalf("event metadata = %+v", eventPayload)
	}
	forwarded := forwarder.snapshot()
	if len(forwarded) != 1 || string(forwarded[0]) != payload {
		t.Fatalf("forwarded = %q, want %q", forwarded, payload)
	}

	cancelRun()
	if err := waitForRunResult(t, runResult); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if count := port.closeCount.Load(); count != 1 {
		t.Fatalf("Close() count = %d, want 1", count)
	}
	if bridge.Status().State != StateStopped {
		t.Fatalf("state = %q, want stopped", bridge.Status().State)
	}
}

func TestBridgeReconnectsAfterFailures(t *testing.T) {
	tests := []struct {
		name        string
		firstResult func() openResult
		trigger     func(*scriptedPort)
	}{
		{
			name: "open failure",
			firstResult: func() openResult {
				return openResult{err: errors.New("device missing")}
			},
		},
		{
			name: "startup write failure",
			firstResult: func() openResult {
				port := newScriptedPort()
				port.failWriteAt = 1
				port.writeError = errors.New("startup write failed")
				return openResult{port: port}
			},
		},
		{
			name: "read failure",
			firstResult: func() openResult {
				return openResult{port: newScriptedPort()}
			},
			trigger: func(port *scriptedPort) {
				port.reads <- portRead{err: errors.New("device lost")}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var firstPort *scriptedPort
			firstResult := tt.firstResult()
			if port, ok := firstResult.port.(*scriptedPort); ok {
				firstPort = port
			}
			secondPort := newScriptedPort()
			opener := newScriptedOpener(firstResult, openResult{port: secondPort})
			bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{}), nil)
			bridge.wait = func(context.Context, time.Duration) bool { return true }
			bridge.jitter = func(delay time.Duration) time.Duration { return delay }

			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() { result <- bridge.Run(ctx) }()

			if tt.trigger != nil {
				waitForBridgeState(t, bridge, StateConnected)
				tt.trigger(firstPort)
			}
			waitForOpenCount(t, opener, 2)
			waitForBridgeState(t, bridge, StateConnected)
			if string(secondPort.writtenBytes()) != enablePacketOutputCommand {
				t.Fatalf("second startup write = %q", secondPort.writtenBytes())
			}
			if firstPort != nil && firstPort.closeCount.Load() != 1 {
				t.Fatalf("first Close() count = %d, want 1", firstPort.closeCount.Load())
			}

			cancel()
			if err := waitForRunResult(t, result); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}

func TestBridgeReconnectsAfterTXFailure(t *testing.T) {
	firstPort := newScriptedPort()
	firstPort.failWriteAt = 2
	firstPort.writeError = errors.New("write failed")
	secondPort := newScriptedPort()
	opener := newScriptedOpener(openResult{port: firstPort}, openResult{port: secondPort})
	bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{}), nil)
	bridge.wait = func(context.Context, time.Duration) bool { return true }

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx) }()
	waitForBridgeState(t, bridge, StateConnected)

	err := bridge.SendText(context.Background(), "*", "Hello", 149)
	if err == nil || !stringsContains(err.Error(), "serial TX") {
		t.Fatalf("SendText() error = %v, want serial TX error", err)
	}
	waitForOpenCount(t, opener, 2)
	waitForBridgeState(t, bridge, StateConnected)
	if firstPort.closeCount.Load() != 1 {
		t.Fatalf("first Close() count = %d, want 1", firstPort.closeCount.Load())
	}

	cancel()
	if err := waitForRunResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestBridgeLogsConnectionLifecycle(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	firstPort := newScriptedPort()
	secondPort := newScriptedPort()
	opener := newScriptedOpener(openResult{port: firstPort}, openResult{port: secondPort})
	bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{}), nil)
	bridge.wait = func(context.Context, time.Duration) bool { return true }
	bridge.jitter = func(delay time.Duration) time.Duration { return delay }

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx) }()
	waitForBridgeState(t, bridge, StateConnected)
	firstPort.reads <- portRead{err: errors.New("device lost")}
	waitForOpenCount(t, opener, 2)
	waitForBridgeState(t, bridge, StateConnected)
	cancel()
	if err := waitForRunResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, message := range []string{
		"msg=\"serial connection attempt\"",
		"msg=\"serial connected\"",
		"msg=\"serial connection lost; retrying\"",
	} {
		if !stringsContains(logs.String(), message) {
			t.Fatalf("logs = %q, missing %q", logs.String(), message)
		}
	}
}

func TestBridgeLogsRawSerialInputAtDebug(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	bridge := newTestBridge(t, newScriptedOpener(), packetingest.NewProcessor(packetingest.Dependencies{}), nil)
	decoder, err := NewDecoder(1024)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}
	raw := []byte("serial diagnostic\\r\\n")

	bridge.processSerialBytes(decoder, raw)

	if !stringsContains(logs.String(), "msg=\"serial RX\"") ||
		!stringsContains(logs.String(), "data=\"serial diagnostic\\\\r\\\\n\"") {
		t.Fatalf("logs = %q, want raw serial input", logs.String())
	}
}

func TestBridgeForwardsMalformedJSONBeforeParseFailure(t *testing.T) {
	port := newScriptedPort()
	opener := newScriptedOpener(openResult{port: port})
	bus := events.NewBus()
	eventContext, cancelEvents := context.WithCancel(context.Background())
	defer cancelEvents()
	subscriber := bus.Subscribe(eventContext)
	forwarder := &recordingForwarder{}
	bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{Bus: bus}), forwarder)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx) }()
	waitForBridgeState(t, bridge, StateConnected)

	port.reads <- portRead{data: []byte("[EXT] Out: {invalid}\n")}
	event := readSerialEvent(t, subscriber)
	if event.Type != "packet.error" {
		t.Fatalf("event type = %q, want packet.error", event.Type)
	}
	forwarded := forwarder.snapshot()
	if len(forwarded) != 1 || string(forwarded[0]) != "{invalid}" {
		t.Fatalf("forwarded = %q, want {invalid}", forwarded)
	}

	cancel()
	if err := waitForRunResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestBridgeIgnoresTypedNilForwarder(t *testing.T) {
	var forwarder *panicForwarder
	bridge := newTestBridge(t, newScriptedOpener(), packetingest.NewProcessor(packetingest.Dependencies{}), forwarder)
	decoder, err := NewDecoder(1024)
	if err != nil {
		t.Fatalf("NewDecoder() error = %v", err)
	}

	payload := `{"src_type":"node","type":"msg","src":"QQ1OWN-1","dst":"QQ1PEER-2","msg":"Hello{663"}`
	bridge.processSerialBytes(decoder, []byte("[EXT] Out: "+payload+" Len: 100\r\n"))
}

func TestBridgeRejectsDisconnectedSendAndSecondRun(t *testing.T) {
	port := newScriptedPort()
	opener := newScriptedOpener(openResult{port: port})
	bridge := newTestBridge(t, opener, packetingest.NewProcessor(packetingest.Dependencies{}), nil)

	if err := bridge.SendText(context.Background(), "*", "Hello", 149); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("SendText() error = %v, want ErrDisconnected", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- bridge.Run(ctx) }()
	waitForBridgeState(t, bridge, StateConnected)
	if err := bridge.Run(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Run() error = %v, want ErrAlreadyRunning", err)
	}
	cancel()
	if err := waitForRunResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestBridgeDryRunValidatesWithoutConnection(t *testing.T) {
	bridge := newTestBridge(
		t,
		newScriptedOpener(),
		packetingest.NewProcessor(packetingest.Dependencies{}),
		nil,
	)
	bridge.disableTX = true

	if err := bridge.SendText(context.Background(), "*", "Hello", 149); err != nil {
		t.Fatalf("SendText() dry-run error = %v", err)
	}
	if err := bridge.SendText(context.Background(), "*", "line\nbreak", 149); !errors.Is(err, ErrCommandInjection) {
		t.Fatalf("SendText() invalid dry-run error = %v, want ErrCommandInjection", err)
	}
}

func TestActiveSessionSerializesPartialWrites(t *testing.T) {
	port := newScriptedPort()
	port.writeLimit = 1
	session := newActiveSession(port)
	start := make(chan struct{})
	errorsChannel := make(chan error, 2)
	for _, payload := range [][]byte{[]byte("AAAA"), []byte("BBBB")} {
		payload := payload
		go func() {
			<-start
			errorsChannel <- session.write(context.Background(), payload)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errorsChannel; err != nil {
			t.Fatalf("write() error = %v", err)
		}
	}
	got := string(port.writtenBytes())
	if got != "AAAABBBB" && got != "BBBBAAAA" {
		t.Fatalf("serialized writes = %q", got)
	}
}

func TestActiveSessionHandlesInvalidWritesAndFailureOnce(t *testing.T) {
	port := newScriptedPort()
	port.writeZero = true
	session := newActiveSession(port)
	if err := session.write(context.Background(), []byte("data")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("write() error = %v, want io.ErrShortWrite", err)
	}

	first := errors.New("first")
	session.fail(first)
	session.fail(errors.New("second"))
	if !errors.Is(session.failureError(), first) {
		t.Fatalf("failure = %v, want first", session.failureError())
	}
	if port.closeCount.Load() != 1 {
		t.Fatalf("Close() count = %d, want 1", port.closeCount.Load())
	}
}

func TestRetryHelpers(t *testing.T) {
	if got := nextDelay(time.Second, 30*time.Second); got != 2*time.Second {
		t.Fatalf("nextDelay() = %s, want 2s", got)
	}
	if got := nextDelay(20*time.Second, 30*time.Second); got != 30*time.Second {
		t.Fatalf("nextDelay() = %s, want 30s", got)
	}
	if got := nextDelay(30*time.Second, 30*time.Second); got != 30*time.Second {
		t.Fatalf("nextDelay() = %s, want 30s", got)
	}
	for range 100 {
		got := jitterDelay(10 * time.Second)
		if got < 8*time.Second || got > 12*time.Second {
			t.Fatalf("jitterDelay() = %s, want [8s,12s]", got)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForRetry(ctx, time.Hour) {
		t.Fatal("waitForRetry() = true after cancellation")
	}
}

func TestNewBridgeValidatesOptions(t *testing.T) {
	valid := Config{
		Port:             PortConfig{Device: "/dev/ttyUSB0"},
		ReconnectInitial: time.Second,
		ReconnectMax:     30 * time.Second,
		StableResetAfter: 30 * time.Second,
		MaxRecordBytes:   1024,
	}
	tests := []struct {
		name      string
		config    Config
		processor *packetingest.Processor
	}{
		{name: "missing processor", config: valid},
		{
			name: "invalid reconnect",
			config: func() Config {
				config := valid
				config.ReconnectInitial = 31 * time.Second
				return config
			}(),
			processor: packetingest.NewProcessor(packetingest.Dependencies{}),
		},
		{
			name: "invalid stable reset",
			config: func() Config {
				config := valid
				config.StableResetAfter = 0
				return config
			}(),
			processor: packetingest.NewProcessor(packetingest.Dependencies{}),
		},
		{
			name: "invalid record limit",
			config: func() Config {
				config := valid
				config.MaxRecordBytes = 0
				return config
			}(),
			processor: packetingest.NewProcessor(packetingest.Dependencies{}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewBridge(Options{Config: tt.config, Processor: tt.processor}); err == nil {
				t.Fatal("NewBridge() error = nil")
			}
		})
	}
}

func newTestBridge(t *testing.T, opener PortOpener, processor *packetingest.Processor, forwarder Forwarder) *Bridge {
	t.Helper()
	bridge, err := NewBridge(Options{
		Config: Config{
			Port: PortConfig{
				Device:      "/dev/ttyUSB0",
				Baud:        115200,
				DataBits:    8,
				Parity:      "none",
				StopBits:    1,
				DTR:         false,
				RTS:         false,
				ReadTimeout: time.Second,
			},
			ReconnectInitial: time.Second,
			ReconnectMax:     30 * time.Second,
			StableResetAfter: 30 * time.Second,
			MaxRecordBytes:   1024,
		},
		Opener:    opener,
		Processor: processor,
		Forwarder: forwarder,
		Identity:  encoderIdentity("QQ1OWN-1"),
	})
	if err != nil {
		t.Fatalf("NewBridge() error = %v", err)
	}
	return bridge
}

func waitForBridgeState(t *testing.T, bridge *Bridge, state State) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if bridge.Status().State == state {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state = %q, want %q", bridge.Status().State, state)
}

func waitForOpenCount(t *testing.T, opener *scriptedOpener, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(opener.configurations()) >= count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("open count = %d, want at least %d", len(opener.configurations()), count)
}

func waitForRunResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
		return nil
	}
}

func readSerialEvent(t *testing.T, subscriber <-chan events.Event) events.Event {
	t.Helper()
	select {
	case event := <-subscriber:
		return event
	case <-time.After(time.Second):
		t.Fatal("event timeout")
		return events.Event{}
	}
}

func stringsContains(value, substring string) bool {
	return bytes.Contains([]byte(value), []byte(substring))
}
