package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

func TestNewPositionPacket(t *testing.T) {
	sender := newTestSender(t)
	options := positionOptions{
		source:     "QQ5SIM-1",
		latitude:   defaultLatitude,
		longitude:  defaultLongitude,
		jitter:     defaultJitter,
		altitude:   12,
		battery:    95,
		hardwareID: "IOT-SIM",
		firmware:   "sim",
	}

	packet := sender.newPositionPacket(options)

	if packet.Type != meshcom.PacketTypePosition {
		t.Fatalf("type = %q, want %q", packet.Type, meshcom.PacketTypePosition)
	}
	if packet.SourceType != "node" {
		t.Fatalf("source type = %q, want node", packet.SourceType)
	}
	if packet.Source != options.source {
		t.Fatalf("source = %q, want %q", packet.Source, options.source)
	}
	assertWithinJitter(t, "latitude", packet.Latitude, options.latitude, options.jitter)
	assertWithinJitter(t, "longitude", packet.Longitude, options.longitude, options.jitter)
	if packet.LatitudeDir != "N" || packet.LongitudeDir != "E" {
		t.Fatalf("directions = %q/%q, want N/E", packet.LatitudeDir, packet.LongitudeDir)
	}
	if !regexp.MustCompile(`^[0-9A-F]{8}$`).MatchString(packet.MessageID) {
		t.Fatalf("message id = %q, want 8 uppercase hex chars", packet.MessageID)
	}
}

func TestNewPositionPacketParsesAsMeshComPosition(t *testing.T) {
	sender := newTestSender(t)
	packet := sender.newPositionPacket(positionOptions{
		source:     "QQ5SIM-1",
		latitude:   defaultLatitude,
		longitude:  defaultLongitude,
		jitter:     0,
		altitude:   12,
		battery:    95,
		hardwareID: "IOT-SIM",
		firmware:   "sim",
	})

	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal position packet: %v", err)
	}

	envelope, err := meshcom.ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
	if envelope.Type != meshcom.PacketTypePosition {
		t.Fatalf("parsed type = %q, want %q", envelope.Type, meshcom.PacketTypePosition)
	}
}

func TestSendDirectMessage(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{myCall: "QQ5ABC-1", directSource: directSource}

	if err := sender.sendDirectMessage(target.addr, options, 7); err != nil {
		t.Fatalf("sendDirectMessage() error = %v", err)
	}

	packet := target.readTextMessage(t)
	if packet.Source != directSource {
		t.Fatalf("source = %q, want %q", packet.Source, directSource)
	}
	if packet.Destination != options.myCall {
		t.Fatalf("destination = %q, want %q", packet.Destination, options.myCall)
	}
	if packet.Message != "DM test 7" {
		t.Fatalf("message = %q, want DM test 7", packet.Message)
	}
}

func TestRunWithoutEnabledAutomaticSends(t *testing.T) {
	target := newUDPReader(t)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	cfg := config{
		listenAddr: "127.0.0.1:0",
		targetAddr: target.addr.String(),
		myCall:     "QQ5ABC-1",
		seed:       1,
	}

	go func() {
		errCh <- run(ctx, cfg)
	}()

	select {
	case err := <-errCh:
		t.Fatalf("run() returned before cancellation: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if _, err := target.readDatagram(20 * time.Millisecond); !errors.Is(err, errReadTimeout) {
		t.Fatalf("readDatagram() error = %v, want timeout", err)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not stop after cancellation")
	}
}

func TestHandleReceivedDatagramDirectResponder(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{myCall: "QQ5ABC-1", directSource: directSource, repeater: repeaterSource}

	handleReceivedDatagram(context.Background(), sender, target.addr, []byte(`{"type":"msg","dst":"QQ1TST-1","msg":"test"}`), options)

	echo := target.readTextMessage(t)
	if echo.SourceType != "node" || echo.Source != options.myCall || echo.Destination != directSource || echo.Message != "test{123" {
		t.Fatalf("echo = %#v", echo)
	}
	ack := target.readTextMessage(t)
	if ack.Source != directSource || ack.Destination != options.myCall || ack.Message != "QQ5ABC-1 :ack123" {
		t.Fatalf("ack = %#v", ack)
	}
}

func TestHandleReceivedDatagramRepeater(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{
		myCall:       "QQ5ABC-1",
		directSource: directSource,
		repeater:     repeaterSource,
		ackDelay:     time.Millisecond,
	}

	handleReceivedDatagram(context.Background(), sender, target.addr, []byte(`{"type":"msg","src":"WRITER-9,RELAY-1","dst":"QQ1TST-2","msg":"repeat me{42"}`), options)

	echo := target.readTextMessage(t)
	if echo.SourceType != "node" || echo.Source != "WRITER-9" || echo.Destination != repeaterSource || echo.Message != "repeat me{42" {
		t.Fatalf("echo = %#v", echo)
	}
	ack := target.readTextMessage(t)
	if ack.Source != repeaterSource || ack.Destination != "WRITER-9" || ack.Message != "WRITER-9 :ack42" {
		t.Fatalf("ack = %#v", ack)
	}
	mirror := target.readTextMessage(t)
	if mirror.Source != repeaterSource || mirror.Destination != "WRITER-9" || mirror.Message != "MIRROR: repeat me" {
		t.Fatalf("mirror = %#v", mirror)
	}
}

func TestHandleReceivedDatagramNumericChannelDestination(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{myCall: "QQ5ABC-1", directSource: directSource, repeater: repeaterSource}

	handleReceivedDatagram(context.Background(), sender, target.addr, []byte(`{"type":"msg","src":"WRITER-9","dst":"7","msg":"hello"}`), options)

	reply := target.readTextMessage(t)
	if reply.Source != options.myCall || reply.Destination != "7" || reply.Message != "hello" {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestHandleReceivedDatagramBroadcastDestination(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{myCall: "QQ5ABC-1", directSource: directSource, repeater: repeaterSource}

	handleReceivedDatagram(context.Background(), sender, target.addr, []byte(`{"type":"msg","dst":"*","msg":"hello"}`), options)

	reply := target.readTextMessage(t)
	if reply.Source != options.myCall || reply.Destination != "*" || reply.Message != "hello" {
		t.Fatalf("reply = %#v", reply)
	}
}

func TestHandleReceivedDatagramIgnoresOtherDestinations(t *testing.T) {
	sender := newTestSender(t)
	target := newUDPReader(t)
	options := messageOptions{myCall: "QQ5ABC-1", directSource: directSource, repeater: repeaterSource}

	handleReceivedDatagram(context.Background(), sender, target.addr, []byte(`{"type":"msg","dst":"OTHER-1","msg":"hello"}`), options)

	if _, err := target.readDatagram(20 * time.Millisecond); !errors.Is(err, errReadTimeout) {
		t.Fatalf("readDatagram() error = %v, want timeout", err)
	}
}

func TestSequenceHelpers(t *testing.T) {
	if got := sequenceID("hello"); got != "123" {
		t.Fatalf("sequenceID() = %q, want 123", got)
	}
	if got := sequenceID("hello{123}"); got != "123" {
		t.Fatalf("sequenceID() = %q, want 123", got)
	}
	if got := messageWithSequence("hello", "5"); got != "hello{5" {
		t.Fatalf("messageWithSequence() = %q, want hello{5", got)
	}
	if got := cleanSequence("hello{5}"); got != "hello" {
		t.Fatalf("cleanSequence() = %q, want hello", got)
	}
}

func TestReplyCallsignUsesIncomingOrigin(t *testing.T) {
	message := meshcom.TextMessage{Source: "writer-9,relay-1"}
	if got := replyCallsign(message, "QQ5ABC-1"); got != "WRITER-9" {
		t.Fatalf("replyCallsign() = %q, want WRITER-9", got)
	}

	message.Source = ""
	if got := replyCallsign(message, "QQ5ABC-1"); got != "QQ5ABC-1" {
		t.Fatalf("replyCallsign() fallback = %q, want QQ5ABC-1", got)
	}
}

func TestNewPositionOptionsUsesRequestedSource(t *testing.T) {
	cfg := config{
		latitude:   defaultLatitude,
		longitude:  defaultLongitude,
		jitter:     defaultJitter,
		altitude:   12,
		battery:    95,
		hardwareID: "IOT-SIM",
		firmware:   "sim",
	}

	first := newPositionOptions(cfg, directSource)
	second := newPositionOptions(cfg, repeaterSource)

	if first.source != directSource {
		t.Fatalf("first source = %q, want %q", first.source, directSource)
	}
	if second.source != repeaterSource {
		t.Fatalf("second source = %q, want %q", second.source, repeaterSource)
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := config{
		targetAddr:        "127.0.0.1:1799",
		myCall:            "QQ5ABC-1",
		position1Interval: time.Second,
		position2Interval: time.Second,
		dmInterval:        time.Second,
		enablePosition1:   true,
		enablePosition2:   true,
		enableDM:          true,
		enableBroadcast:   true,
		enableChannel2:    true,
	}
	tests := map[string]struct {
		cfg     config
		wantErr bool
	}{
		"valid": {
			cfg: validConfig,
		},
		"no enabled auto sends allows zero intervals": {
			cfg: config{targetAddr: "127.0.0.1:1799", myCall: "QQ5ABC-1"},
		},
		"no enabled auto sends still rejects negative jitter": {
			cfg:     config{targetAddr: "127.0.0.1:1799", myCall: "QQ5ABC-1", jitter: -0.1},
			wantErr: true,
		},
		"no enabled auto sends still rejects negative ack delay": {
			cfg:     config{targetAddr: "127.0.0.1:1799", myCall: "QQ5ABC-1", ackDelay: -time.Second},
			wantErr: true,
		},
		"missing target": {
			cfg:     func() config { cfg := validConfig; cfg.targetAddr = ""; return cfg }(),
			wantErr: true,
		},
		"missing my call": {
			cfg:     func() config { cfg := validConfig; cfg.myCall = ""; return cfg }(),
			wantErr: true,
		},
		"negative jitter": {
			cfg:     func() config { cfg := validConfig; cfg.jitter = -0.1; return cfg }(),
			wantErr: true,
		},
		"zero first position interval": {
			cfg:     func() config { cfg := validConfig; cfg.position1Interval = 0; return cfg }(),
			wantErr: true,
		},
		"zero second position interval": {
			cfg:     func() config { cfg := validConfig; cfg.position2Interval = 0; return cfg }(),
			wantErr: true,
		},
		"zero dm interval": {
			cfg:     func() config { cfg := validConfig; cfg.dmInterval = 0; return cfg }(),
			wantErr: true,
		},
		"negative ack delay": {
			cfg:     func() config { cfg := validConfig; cfg.ackDelay = -time.Second; return cfg }(),
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateConfig(test.cfg)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateConfig() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func assertWithinJitter(t *testing.T, name string, got float64, base float64, jitter float64) {
	t.Helper()
	if math.Abs(got-base) > jitter {
		t.Fatalf("%s = %f, want within %f of %f", name, got, jitter, base)
	}
}

type udpReader struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

var errReadTimeout = errors.New("read timeout")

func newTestSender(t *testing.T) *udpSender {
	t.Helper()
	conn, err := net.ListenUDP("udp", mustResolveUDPAddr(t, "127.0.0.1:0"))
	if err != nil {
		t.Fatalf("listen sender UDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &udpSender{conn: conn, random: rand.New(rand.NewSource(1))}
}

func newUDPReader(t *testing.T) *udpReader {
	t.Helper()
	conn, err := net.ListenUDP("udp", mustResolveUDPAddr(t, "127.0.0.1:0"))
	if err != nil {
		t.Fatalf("listen target UDP: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &udpReader{conn: conn, addr: conn.LocalAddr().(*net.UDPAddr)}
}

func mustResolveUDPAddr(t *testing.T, addr string) *net.UDPAddr {
	t.Helper()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve UDP addr: %v", err)
	}
	return udpAddr
}

func (r *udpReader) readTextMessage(t *testing.T) meshcom.TextMessage {
	t.Helper()
	data, err := r.readDatagram(time.Second)
	if err != nil {
		t.Fatalf("readDatagram() error = %v", err)
	}
	envelope, err := meshcom.ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
	message, ok := envelope.Packet.(meshcom.TextMessage)
	if !ok {
		t.Fatalf("packet = %T, want TextMessage", envelope.Packet)
	}
	return message
}

func (r *udpReader) readDatagram(timeout time.Duration) ([]byte, error) {
	if err := r.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	buf := make([]byte, 2048)
	n, _, err := r.conn.ReadFromUDP(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, errReadTimeout
		}
		return nil, err
	}
	return append([]byte(nil), buf[:n]...), nil
}
