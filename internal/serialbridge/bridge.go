package serialbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	rand "math/rand/v2"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/packetingest"
	"github.com/logocomune/gomeshcom-client/internal/transport"
)

const enablePacketOutputCommand = "--setinfo on\r\n"

var (
	ErrDisconnected   = fmt.Errorf("serial transport is disconnected: %w", transport.ErrUnavailable)
	ErrAlreadyRunning = errors.New("serial transport is already running")
)

type State = transport.State

const (
	StateDisconnected = transport.StateDisconnected
	StateConnecting   = transport.StateConnecting
	StateConnected    = transport.StateConnected
	StateDegraded     = transport.StateDegraded
	StateStopped      = transport.StateStopped
)

type Status = transport.Status

type Config struct {
	Port             PortConfig
	ReconnectInitial time.Duration
	ReconnectMax     time.Duration
	StableResetAfter time.Duration
	MaxRecordBytes   int
}

type Forwarder interface {
	Forward(data []byte)
}

type Options struct {
	Config    Config
	Opener    PortOpener
	Processor *packetingest.Processor
	Forwarder Forwarder
	Identity  Identity
	DisableTX bool
}

type Bridge struct {
	config    Config
	opener    PortOpener
	processor *packetingest.Processor
	forwarder Forwarder
	encoder   *Encoder
	disableTX bool

	running atomic.Bool

	statusMu sync.RWMutex
	status   Status

	sessionMu sync.RWMutex
	session   *activeSession

	now    func() time.Time
	wait   func(context.Context, time.Duration) bool
	jitter func(time.Duration) time.Duration
}

func NewBridge(options Options) (*Bridge, error) {
	if options.Processor == nil {
		return nil, errors.New("serial packet processor is required")
	}
	if options.Config.ReconnectInitial <= 0 ||
		options.Config.ReconnectMax <= 0 ||
		options.Config.ReconnectInitial > options.Config.ReconnectMax {
		return nil, errors.New("invalid serial reconnect configuration")
	}
	if options.Config.StableResetAfter <= 0 {
		return nil, errors.New("serial stable reset duration must be greater than zero")
	}
	if options.Config.MaxRecordBytes <= 0 {
		return nil, ErrInvalidRecordLimit
	}
	opener := options.Opener
	if opener == nil {
		opener = LibraryPortOpener{}
	}
	return &Bridge{
		config:    options.Config,
		opener:    opener,
		processor: options.Processor,
		forwarder: normalizeForwarder(options.Forwarder),
		encoder:   NewEncoder(options.Identity),
		disableTX: options.DisableTX,
		status: Status{
			Mode:     "serial",
			State:    StateDisconnected,
			Endpoint: options.Config.Port.Device,
		},
		now:    time.Now,
		wait:   waitForRetry,
		jitter: jitterDelay,
	}, nil
}

func normalizeForwarder(forwarder Forwarder) Forwarder {
	if forwarder == nil {
		return nil
	}
	value := reflect.ValueOf(forwarder)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return nil
		}
	}
	return forwarder
}

func (b *Bridge) Run(ctx context.Context) error {
	if !b.running.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer func() {
		b.running.Store(false)
		b.setStopped()
	}()

	delay := b.config.ReconnectInitial
	var retryCount uint64
	for {
		if ctx.Err() != nil {
			return nil
		}
		slog.Info("serial connection attempt", "device", b.config.Port.Device, "attempt", retryCount+1)
		b.setConnecting(retryCount)
		startedAt := b.now()
		err := b.runSession(ctx)
		if ctx.Err() != nil {
			return nil
		}

		retryCount++
		b.setDegraded(err, retryCount)
		if b.now().Sub(startedAt) >= b.config.StableResetAfter {
			delay = b.config.ReconnectInitial
		}
		retryDelay := b.jitter(delay)
		slog.Warn("serial connection lost; retrying", "device", b.config.Port.Device, "error", err, "retry_in", retryDelay)
		if !b.wait(ctx, retryDelay) {
			return nil
		}
		delay = nextDelay(delay, b.config.ReconnectMax)
	}
}

func (b *Bridge) SendText(ctx context.Context, destination string, message string, maxLength int) error {
	command, err := b.encoder.Encode(TextCommand{
		Destination:     destination,
		Message:         message,
		MaxMessageRunes: maxLength,
	})
	if err != nil {
		return err
	}
	if b.disableTX {
		slog.Warn("serial TX disabled (dry-run)", "destination", destination)
		return nil
	}
	session := b.connectedSession()
	if session == nil {
		return ErrDisconnected
	}
	if err := session.write(ctx, command); err != nil {
		session.fail(err)
		b.setDegraded(err, b.Status().RetryCount)
		return fmt.Errorf("serial TX: %w", err)
	}
	return nil
}

func (b *Bridge) Status() Status {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.status
}

func (b *Bridge) TransportStatus() transport.Status {
	return b.Status()
}

func (b *Bridge) runSession(ctx context.Context) error {
	port, err := b.opener.Open(b.config.Port)
	if err != nil {
		return err
	}
	decoder, err := NewDecoder(b.config.MaxRecordBytes)
	if err != nil {
		_ = port.Close()
		return err
	}

	session := newActiveSession(port)
	b.installSession(session)
	defer b.removeSession(session)

	stopWatcher := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		select {
		case <-ctx.Done():
			session.closePort()
		case <-stopWatcher:
		}
	}()
	defer func() {
		close(stopWatcher)
		session.closePort()
		watcher.Wait()
	}()

	if err := session.write(ctx, []byte(enablePacketOutputCommand)); err != nil {
		return fmt.Errorf("enable serial packet output: %w", err)
	}
	b.setConnected()
	slog.Info("serial connected",
		"device", b.config.Port.Device,
		"baud", b.config.Port.Baud,
		"data_bits", b.config.Port.DataBits,
		"parity", b.config.Port.Parity,
		"stop_bits", b.config.Port.StopBits,
	)

	buffer := make([]byte, 4096)
	for {
		bytesRead, readErr := port.Read(buffer)
		if bytesRead > 0 {
			b.processSerialBytes(decoder, buffer[:bytesRead])
		}
		if failure := session.failureError(); failure != nil {
			return failure
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(readErr, io.EOF) {
				return io.EOF
			}
			return fmt.Errorf("read serial device: %w", readErr)
		}
	}
}

func (b *Bridge) processSerialBytes(decoder *Decoder, data []byte) {
	slog.Debug("serial RX", "device", b.config.Port.Device, "bytes", len(data), "data", string(data))
	result := decoder.Feed(data)
	for _, decodeError := range result.Errors {
		slog.Warn("serial record ignored", "device", b.config.Port.Device, "error", decodeError)
	}
	for _, payload := range result.Payloads {
		if b.forwarder != nil {
			b.forwarder.Forward(payload)
		}
		_ = b.processor.Process(packetingest.Source{
			Transport: "serial",
			Endpoint:  b.config.Port.Device,
		}, payload)
	}
}

func (b *Bridge) connectedSession() *activeSession {
	if b.Status().State != StateConnected {
		return nil
	}
	b.sessionMu.RLock()
	defer b.sessionMu.RUnlock()
	return b.session
}

func (b *Bridge) installSession(session *activeSession) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	b.session = session
}

func (b *Bridge) removeSession(session *activeSession) {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if b.session == session {
		b.session = nil
	}
}

func (b *Bridge) setConnecting(retryCount uint64) {
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	b.status.State = StateConnecting
	b.status.RetryCount = retryCount
	b.status.ConnectedAt = nil
}

func (b *Bridge) setConnected() {
	now := b.now().UTC()
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	b.status.State = StateConnected
	b.status.LastError = ""
	b.status.LastErrorAt = nil
	b.status.ConnectedAt = &now
}

func (b *Bridge) setDegraded(err error, retryCount uint64) {
	now := b.now().UTC()
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	b.status.State = StateDegraded
	b.status.ConnectedAt = nil
	b.status.RetryCount = retryCount
	b.status.LastErrorAt = &now
	if err == nil {
		b.status.LastError = ErrDisconnected.Error()
		return
	}
	b.status.LastError = err.Error()
}

func (b *Bridge) setStopped() {
	b.statusMu.Lock()
	defer b.statusMu.Unlock()
	b.status.State = StateStopped
	b.status.ConnectedAt = nil
}

type activeSession struct {
	port Port

	writeMu sync.Mutex
	close   sync.Once
	failMu  sync.RWMutex
	failure error
}

func newActiveSession(port Port) *activeSession {
	return &activeSession{port: port}
}

func (s *activeSession) write(ctx context.Context, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		written, err := s.port.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 || written > len(data) {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func (s *activeSession) fail(err error) {
	s.failMu.Lock()
	if s.failure == nil {
		s.failure = err
	}
	s.failMu.Unlock()
	s.closePort()
}

func (s *activeSession) failureError() error {
	s.failMu.RLock()
	defer s.failMu.RUnlock()
	return s.failure
}

func (s *activeSession) closePort() {
	s.close.Do(func() {
		_ = s.port.Close()
	})
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func jitterDelay(delay time.Duration) time.Duration {
	const jitterPercent = 20
	window := delay * jitterPercent / 100
	if window <= 0 {
		return delay
	}
	offset := time.Duration(rand.Int64N(int64(window)*2+1)) - window
	return delay + offset
}

func nextDelay(current, maximum time.Duration) time.Duration {
	if current >= maximum/2 {
		return maximum
	}
	return current * 2
}
