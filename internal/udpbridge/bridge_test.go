package udpbridge

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/positions"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
	"github.com/logocomune/gomeshcom-client/internal/storage"
	"github.com/logocomune/gomeshcom-client/internal/telemetry"
	"github.com/logocomune/gomeshcom-client/internal/udpforward"
	_ "modernc.org/sqlite"
)

// staticCallsign is a test-only myCallSource returning a fixed callsign.
type staticCallsign string

func (s staticCallsign) Current() string { return string(s) }

type recordedIncoming struct {
	conversationID string
	message        string
}

type recordingChatStatus struct {
	records []recordedIncoming
}

func (s *recordingChatStatus) RecordIncoming(conversationID string, _ time.Time, message string) {
	s.records = append(s.records, recordedIncoming{
		conversationID: conversationID,
		message:        message,
	})
}

func newBridgeReceiveLog(t *testing.T, cfg receivelog.Config) *receivelog.Logger {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "receive.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE receive_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			received_at TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			bytes INTEGER NOT NULL,
			raw TEXT NOT NULL,
			packet_type TEXT,
			parse_error TEXT
		)
	`); err != nil {
		t.Fatalf("create receive_log table: %v", err)
	}
	return receivelog.NewSQLite(cfg, db)
}

func newBridgeChatLog(t *testing.T, identity chatlogSource) *chatlog.Logger {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE chats_dm (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, msg_id TEXT, sequence_id TEXT, received_at TEXT NOT NULL, src TEXT, src_type TEXT, via TEXT, dst TEXT NOT NULL, msg TEXT NOT NULL, rssi INTEGER, snr INTEGER, direction TEXT, delivery_status TEXT, ack_status TEXT, ack_received_at TEXT, ack_src TEXT, ack_src_type TEXT, ack_rssi INTEGER, ack_snr INTEGER, ack_via TEXT)`,
		`CREATE TABLE chats_public (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, kind TEXT NOT NULL, channel TEXT, msg_id TEXT, received_at TEXT NOT NULL, src TEXT, src_type TEXT, via TEXT, dst TEXT NOT NULL, msg TEXT NOT NULL, rssi INTEGER, snr INTEGER)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("create chatlog table: %v", err)
		}
	}
	return chatlog.NewSQLite(db, identity)
}

type chatlogSource interface {
	Current() string
}

func newBridgePositionsStore(t *testing.T) *positions.Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gomeshcom.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE nodes (
			node_id TEXT PRIMARY KEY, lat REAL, lng REAL, alt INTEGER, hw_id TEXT,
			firstseen TEXT, lastseen TEXT, lastdirectseen TEXT, rssi INTEGER, snr INTEGER, via TEXT
		)
	`); err != nil {
		t.Fatalf("create nodes: %v", err)
	}
	return positions.NewSQLite(db)
}

func newBridgeTelemetryStore(t *testing.T) (*telemetry.Store, *storage.DB) {
	t.Helper()
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "telemetry.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return telemetry.NewSQLite(db.SQL()), db
}

func TestHandleDatagramLogsValidPacket(t *testing.T) {
	bus := events.NewBus()
	receiveLogger := newBridgeReceiveLog(t, receivelog.Config{Enabled: true})
	chatLogger := newBridgeChatLog(t, nil)
	bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, receiveLogger, chatLogger, nil, false, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscriber := bus.Subscribe(ctx)

	raw := `{"type":"msg","dst":"*","msg":"hello"}`
	bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)

	event := readEvent(t, subscriber)
	if event.Type != "packet.received" {
		t.Fatalf("event type = %q, want packet.received", event.Type)
	}

	payload, ok := event.Data.(map[string]any)
	if !ok {
		t.Fatalf("event data type = %T, want map", event.Data)
	}
	receivedAt, ok := payload["received_at"].(string)
	if !ok || receivedAt == "" {
		t.Fatalf("event received_at = %#v, want non-empty string", payload["received_at"])
	}
	if payload["remote_addr"] != "127.0.0.1:1799" {
		t.Fatalf("event remote_addr = %#v, want 127.0.0.1:1799", payload["remote_addr"])
	}
	packet, ok := payload["packet"].(meshcom.TextMessage)
	if !ok {
		t.Fatalf("event packet type = %T, want meshcom.TextMessage", payload["packet"])
	}
	if packet.Destination != "*" || packet.Message != "hello" {
		t.Fatalf("event packet = %+v, want broadcast hello", packet)
	}
	if _, err := time.Parse(time.RFC3339Nano, receivedAt); err != nil {
		t.Fatalf("event received_at parse: %v", err)
	}

	chatRecords, err := chatLogger.ReadSince("P_broadcast", time.Time{})
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(chatRecords) != 1 || chatRecords[0].ReceivedAt.Format(time.RFC3339Nano) != receivedAt {
		t.Fatalf("chat records = %+v, want received_at %s", chatRecords, receivedAt)
	}

	records, err := receiveLogger.ReadSince(time.Time{})
	if err != nil {
		t.Fatalf("ReadSince receive log: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("receive log records = %d, want 1", len(records))
	}
	record := records[0]
	if record.PacketType != "msg" {
		t.Fatalf("packet type = %q, want msg", record.PacketType)
	}
	if record.Raw != raw {
		t.Fatalf("raw = %q, want %q", record.Raw, raw)
	}
}

func TestHandleDatagramLogsParseError(t *testing.T) {
	bus := events.NewBus()
	receiveLogger := newBridgeReceiveLog(t, receivelog.Config{Enabled: true})
	bridge := NewBridge("127.0.0.1:0", "", bus, receiveLogger, nil, nil, false, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscriber := bus.Subscribe(ctx)

	raw := `{`
	bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)

	event := readEvent(t, subscriber)
	if event.Type != "packet.error" {
		t.Fatalf("event type = %q, want packet.error", event.Type)
	}
	if parseError, ok := event.Data.(string); !ok || parseError == "" {
		t.Fatalf("event data = %#v, want non-empty parse error", event.Data)
	}
	if _, err := bridge.effectiveNodeAddr(); err != ErrNodeNotDetected {
		t.Fatalf("invalid packet learned node address: %v", err)
	}

	records, err := receiveLogger.ReadSince(time.Time{})
	if err != nil {
		t.Fatalf("ReadSince receive log: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("receive log records = %d, want 1", len(records))
	}
	record := records[0]
	if record.ParseError == "" {
		t.Fatal("parse error empty")
	}
}

func TestHandleDatagramStoresTelemetry(t *testing.T) {
	bus := events.NewBus()
	store, db := newBridgeTelemetryStore(t)
	bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, nil, nil, nil, false, nil, nil, nil)
	bridge.SetTelemetryStore(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscriber := bus.Subscribe(ctx)

	raw := `{"src_type":"lora","type":"tele","src":"QQ1ABC-1","batt":90,"temp1":21.5,"rssi":-102,"snr":4}`
	bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)

	event := readEvent(t, subscriber)
	if event.Type != "packet.received" {
		t.Fatalf("event type = %q, want packet.received", event.Type)
	}
	payload, ok := event.Data.(map[string]any)
	if !ok {
		t.Fatalf("event data type = %T, want map", event.Data)
	}
	if _, ok := payload["packet"].(meshcom.Telemetry); !ok {
		t.Fatalf("event packet type = %T, want meshcom.Telemetry", payload["packet"])
	}

	var metric string
	var value float64
	if err := db.SQL().QueryRow(`
		SELECT metric, value FROM telemetry_samples
		WHERE src_origin = 'QQ1ABC-1' AND metric = 'temp1'
	`).Scan(&metric, &value); err != nil {
		t.Fatalf("query telemetry sample: %v", err)
	}
	if metric != "temp1" || value != 21.5 {
		t.Fatalf("telemetry sample = %s/%v, want temp1/21.5", metric, value)
	}

	var rssi, snr int
	if err := db.SQL().QueryRow(`
		SELECT rssi, snr FROM telemetry_direct_signal
		WHERE src_origin = 'QQ1ABC-1'
	`).Scan(&rssi, &snr); err != nil {
		t.Fatalf("query telemetry signal: %v", err)
	}
	if rssi != -102 || snr != 4 {
		t.Fatalf("telemetry signal = %d/%d, want -102/4", rssi, snr)
	}
}

func TestHandleDatagramChatStatusBehavior(t *testing.T) {
	tests := []struct {
		name        string
		identity    string
		raw         string
		wantRecords []recordedIncoming
	}{
		{
			name:     "incoming direct message records unread status",
			identity: "QQ1OWN-1",
			raw:      `{"type":"msg","src":"QQ1PEER-2","dst":"QQ1OWN-1","msg":"hello"}`,
			wantRecords: []recordedIncoming{{
				conversationID: "DM_QQ1OWN-1_QQ1PEER-2",
				message:        "hello",
			}},
		},
		{
			name:     "own device echo does not record unread status",
			identity: "QQ1OWN-1",
			raw:      `{"type":"msg","src":"QQ1OWN-2","dst":"QQ1PEER-2","msg":"hello{123"}`,
		},
		{
			name:     "ack does not record unread status",
			identity: "QQ1OWN-1",
			raw:      `{"type":"msg","src":"QQ1PEER-2","dst":"QQ1OWN-1","msg":"QQ1OWN-1:ack123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &recordingChatStatus{}
			chatLogger := newBridgeChatLog(t, staticCallsign(tt.identity))
			bridge := NewBridge(
				"127.0.0.1:0",
				"127.0.0.1:1799",
				events.NewBus(),
				nil,
				chatLogger,
				nil,
				false,
				nil,
				staticCallsign(tt.identity),
				status,
			)

			bridge.handleDatagram("127.0.0.1:1799", []byte(tt.raw), tt.raw)

			if len(status.records) != len(tt.wantRecords) {
				t.Fatalf("status records = %+v, want %+v", status.records, tt.wantRecords)
			}
			for i := range tt.wantRecords {
				if status.records[i] != tt.wantRecords[i] {
					t.Fatalf("status record %d = %+v, want %+v", i, status.records[i], tt.wantRecords[i])
				}
			}
		})
	}
}

func TestHandleDatagramUpdatesPositionStore(t *testing.T) {
	t.Run("direct packet writes rssi/snr on origin", func(t *testing.T) {
		bus := events.NewBus()
		store := newBridgePositionsStore(t)
		bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, nil, nil, store, false, nil, nil, nil)

		raw := `{"src_type":"node","type":"pos","src":"QQ1ABC-1","msg":"","lat":48.1,"long":16.3,"alt":123,"rssi":-90,"snr":8}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)

		record := store.Snapshot()["QQ1ABC-1"]
		if record.Latitude != 48.1 || record.Longitude != 16.3 || record.Altitude != 123 {
			t.Fatalf("position = %+v", record)
		}
		if record.RSSI != -90 || record.SNR != 8 {
			t.Fatalf("direct: rssi/snr not written: %+v", record)
		}
		if record.LastDirectSeen == nil {
			t.Fatal("direct: LastDirectSeen must be set")
		}
	})

	t.Run("indirect packet: origin keeps zero rssi/snr, relay gets freshness", func(t *testing.T) {
		bus := events.NewBus()
		store := newBridgePositionsStore(t)
		bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, nil, nil, store, false, nil, nil, nil)

		// Pre-populate relay via direct pos.
		relayRaw := `{"type":"pos","src":"RELAY-1","lat":44.0,"long":11.0,"rssi":-70,"snr":2}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(relayRaw), relayRaw)

		// Indirect pos from origin via relay.
		raw := `{"src_type":"node","type":"pos","src":"QQ1ABC-1,RELAY-1","msg":"","lat":48.1,"long":16.3,"alt":123,"rssi":-90,"snr":8}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)

		origin := store.Snapshot()["QQ1ABC-1"]
		if origin.Latitude != 48.1 || origin.Longitude != 16.3 {
			t.Fatalf("origin coords wrong: %+v", origin)
		}
		if origin.RSSI != 0 || origin.SNR != 0 {
			t.Fatalf("indirect origin must not get rssi/snr, got %d/%d", origin.RSSI, origin.SNR)
		}
		if origin.LastDirectSeen != nil {
			t.Fatalf("indirect origin must not get LastDirectSeen, got %v", origin.LastDirectSeen)
		}

		relay := store.Snapshot()["RELAY-1"]
		if relay.LastDirectSeen == nil {
			t.Fatal("relay must get LastDirectSeen from indirect packet")
		}
		if relay.RSSI != -90 || relay.SNR != 8 {
			t.Fatalf("relay must get rssi/snr from indirect packet, got %d/%d", relay.RSSI, relay.SNR)
		}
	})

	t.Run("msg packet touches freshness of existing node", func(t *testing.T) {
		bus := events.NewBus()
		store := newBridgePositionsStore(t)
		bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, nil, nil, store, false, nil, nil, nil)

		// Pre-populate origin.
		posRaw := `{"type":"pos","src":"QQ1ABC-1","lat":48.1,"long":16.3,"rssi":-70,"snr":2}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(posRaw), posRaw)

		// Direct msg → should update freshness.
		msgRaw := `{"type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello","rssi":-80,"snr":5}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(msgRaw), msgRaw)

		record := store.Snapshot()["QQ1ABC-1"]
		if record.RSSI != -80 || record.SNR != 5 {
			t.Fatalf("msg should update rssi/snr: %+v", record)
		}
	})

	t.Run("msg packet without signal preserves existing rssi/snr", func(t *testing.T) {
		bus := events.NewBus()
		store := newBridgePositionsStore(t)
		bridge := NewBridge("127.0.0.1:0", "127.0.0.1:1799", bus, nil, nil, store, false, nil, nil, nil)

		posRaw := `{"type":"pos","src":"QQ1ABC-1","lat":48.1,"long":16.3,"rssi":-70,"snr":2}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(posRaw), posRaw)

		msgRaw := `{"type":"msg","src":"QQ1ABC-1","dst":"*","msg":"hello"}`
		bridge.handleDatagram("127.0.0.1:1799", []byte(msgRaw), msgRaw)

		record := store.Snapshot()["QQ1ABC-1"]
		if record.RSSI != -70 || record.SNR != 2 {
			t.Fatalf("msg without signal should preserve rssi/snr: %+v", record)
		}
		if record.LastDirectSeen == nil {
			t.Fatal("msg without signal should still touch direct freshness")
		}
	})
}

func TestListen(t *testing.T) {
	bus := events.NewBus()
	bridge := NewBridge("127.0.0.1:0", "127.0.0.1:0", bus, nil, nil, nil, false, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscriber := bus.Subscribe(ctx)

	// Get the actually bound address
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	listenAddr := conn.LocalAddr().String()
	conn.Close()

	bridge.listenAddr = listenAddr

	errCh := make(chan error, 1)
	go func() {
		errCh <- bridge.Listen(ctx)
	}()

	// Wait a bit for listener to start
	time.Sleep(50 * time.Millisecond)

	sender, err := net.Dial("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	raw := `{"type":"msg","dst":"*","msg":"hello"}`
	if _, err := sender.Write([]byte(raw)); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-subscriber:
		if event.Type != "packet.received" {
			t.Errorf("event type = %q, want packet.received", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	cancel()
	err = <-errCh
	if err != nil {
		t.Errorf("Listen returned error: %v", err)
	}
}

func TestSendText(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	nodeAddr := conn.LocalAddr().String()

	bridge := NewBridge("127.0.0.1:0", nodeAddr, events.NewBus(), nil, nil, nil, false, nil, nil, nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- bridge.SendText(context.Background(), "*", "hello world", 149)
	}()

	buffer := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatal(err)
	}

	var msg map[string]any
	if err := json.Unmarshal(buffer[:n], &msg); err != nil {
		t.Fatal(err)
	}

	if msg["msg"] != "hello world" {
		t.Errorf("msg = %v, want hello world", msg["msg"])
	}

	err = <-errCh
	if err != nil {
		t.Errorf("SendText returned error: %v", err)
	}
}

func TestEffectiveNodeAddr(t *testing.T) {
	tests := []struct {
		name        string
		nodeAddr    string
		learnedAddr string
		wantAddr    string
		wantErr     bool
	}{
		{
			name:     "explicit config wins",
			nodeAddr: "192.168.0.2:1799",
			wantAddr: "192.168.0.2:1799",
		},
		{
			name:        "explicit config wins even when learned present",
			nodeAddr:    "192.168.0.2:1799",
			learnedAddr: "10.0.0.5:1799",
			wantAddr:    "192.168.0.2:1799",
		},
		{
			name:        "auto-detect uses learned addr",
			nodeAddr:    "",
			learnedAddr: "10.0.0.5:1799",
			wantAddr:    "10.0.0.5:1799",
		},
		{
			name:     "auto-detect no packets yet returns error",
			nodeAddr: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bridge{nodeAddr: tt.nodeAddr}
			if tt.learnedAddr != "" {
				b.learnedNodeAddr.Store(&tt.learnedAddr)
			}
			got, err := b.effectiveNodeAddr()
			if (err != nil) != tt.wantErr {
				t.Fatalf("effectiveNodeAddr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.wantAddr {
				t.Fatalf("effectiveNodeAddr() = %q, want %q", got, tt.wantAddr)
			}
		})
	}
}

func TestHandleDatagramLearnsNodeAddr(t *testing.T) {
	bus := events.NewBus()
	bridge := NewBridge("127.0.0.1:0", "", bus, nil, nil, nil, false, nil, nil, nil)

	// No packets yet — should return ErrNodeNotDetected.
	if _, err := bridge.effectiveNodeAddr(); err == nil {
		t.Fatal("expected ErrNodeNotDetected before any packet")
	}

	// Send a valid packet from a known remote addr.
	raw := `{"type":"msg","dst":"*","msg":"hello"}`
	bridge.handleDatagram("10.0.0.5:1799", []byte(raw), raw)

	got, err := bridge.effectiveNodeAddr()
	if err != nil {
		t.Fatalf("effectiveNodeAddr() after packet: %v", err)
	}
	if got != "10.0.0.5:1799" {
		t.Fatalf("effectiveNodeAddr() = %q, want 10.0.0.5:1799", got)
	}
}

func TestHandleDatagramDoesNotLearnWhenNodeAddrExplicit(t *testing.T) {
	bus := events.NewBus()
	bridge := NewBridge("127.0.0.1:0", "192.168.0.2:1799", bus, nil, nil, nil, false, nil, nil, nil)

	raw := `{"type":"msg","dst":"*","msg":"hello"}`
	bridge.handleDatagram("10.0.0.99:1799", []byte(raw), raw)

	// learnedNodeAddr should be nil — explicit config wins so we never write it.
	if p := bridge.learnedNodeAddr.Load(); p != nil {
		t.Fatalf("learnedNodeAddr should be nil when nodeAddr is explicit, got %q", *p)
	}
}

func TestSendTextAutoDetect(t *testing.T) {
	// Start a UDP receiver to act as the meshcom node.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	nodeAddr := conn.LocalAddr().String()

	// Bridge with empty nodeAddr — must learn before sending.
	bridge := NewBridge("127.0.0.1:0", "", events.NewBus(), nil, nil, nil, false, nil, nil, nil)

	// Before learning: SendText must return ErrNodeNotDetected.
	err = bridge.SendText(context.Background(), "*", "hi", 149)
	if err == nil || err != ErrNodeNotDetected {
		t.Fatalf("expected ErrNodeNotDetected, got %v", err)
	}

	// Simulate learning via an incoming packet from the node.
	bridge.learnedNodeAddr.Store(&nodeAddr)

	// Now sending should work.
	errCh := make(chan error, 1)
	go func() {
		errCh <- bridge.SendText(context.Background(), "*", "hi", 149)
	}()

	buf := make([]byte, 512)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read from udp: %v", err)
	}

	var msg map[string]any
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg["msg"] != "hi" {
		t.Fatalf("msg = %v, want hi", msg["msg"])
	}

	if err := <-errCh; err != nil {
		t.Fatalf("SendText: %v", err)
	}
}

func readEvent(t *testing.T, subscriber <-chan events.Event) events.Event {
	t.Helper()

	select {
	case event := <-subscriber:
		return event
	case <-time.After(time.Second):
		t.Fatal("event timeout")
		return events.Event{}
	}
}

func TestSendTextDryRun(t *testing.T) {
	// Listener simulating a meshcom node — must NOT receive any datagram.
	node, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	nodeAddr := node.LocalAddr().String()

	bridge := NewBridge("127.0.0.1:0", nodeAddr, events.NewBus(), nil, nil, nil, true, nil, nil, nil)

	err = bridge.SendText(context.Background(), "*", "hello dry run", 149)
	if err != nil {
		t.Fatalf("SendText with dry-run returned error: %v", err)
	}

	// Short deadline: any packet arriving means the guard failed.
	_ = node.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf := make([]byte, 512)
	n, _, readErr := node.ReadFromUDP(buf)
	if readErr == nil {
		t.Fatalf("dry-run must not transmit; got %d bytes: %s", n, buf[:n])
	}
}

func TestSendTextDryRunNoNodeRequired(t *testing.T) {
	// Bridge with no node addr and no learned addr — dry-run must still succeed.
	bridge := NewBridge("127.0.0.1:0", "", events.NewBus(), nil, nil, nil, true, nil, nil, nil)

	err := bridge.SendText(context.Background(), "*", "dry run no node", 149)
	if err != nil {
		t.Fatalf("dry-run with no node addr returned error: %v", err)
	}
}

func TestListenForwardsRawDatagram(t *testing.T) {
	// Sink receives forwarded bytes.
	sink, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	sinkAddr := sink.LocalAddr().String()

	fwd, err := udpforward.New([]string{sinkAddr})
	if err != nil {
		t.Fatalf("udpforward.New: %v", err)
	}
	defer fwd.Close()

	// Pre-allocate the listen port so we know where to send.
	tmp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	listenAddr := tmp.LocalAddr().String()
	tmp.Close()

	bus := events.NewBus()
	bridge := NewBridge(listenAddr, "127.0.0.1:0", bus, nil, nil, nil, false, fwd, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- bridge.Listen(ctx) }()
	time.Sleep(30 * time.Millisecond)

	// Send a deliberately malformed packet so we don't need a valid meshcom parser.
	payload := []byte(`malformed-but-forwarded-bytes`)
	sender, err := net.Dial("udp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	if _, err := sender.Write(payload); err != nil {
		t.Fatal(err)
	}

	_ = sink.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 512)
	n, _, err := sink.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("sink did not receive forwarded datagram: %v", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Fatalf("forwarded payload = %q, want %q", buf[:n], payload)
	}
}

func FuzzHandleDatagram(f *testing.F) {
	seeds := []string{
		`{"type":"msg","dst":"*","msg":"hello"}`,
		`{"type":"pos","src":"QQ1ABC-1","lat":48.1,"long":16.3}`,
		`{`,
		``,
		`not json`,
		"\x00\x01\x02",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		bus := events.NewBus()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// Subscribe to drain the bus so it never blocks.
		_ = bus.Subscribe(ctx)

		bridge := NewBridge("127.0.0.1:0", "127.0.0.1:0", bus, nil, nil, nil, false, nil, nil, nil)
		// Must never panic.
		bridge.handleDatagram("127.0.0.1:1799", []byte(raw), raw)
	})
}

func readChatRecord(t *testing.T, path string) chatlog.Record {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open chat log: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("chat log empty")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat log: %v", err)
	}

	var record chatlog.Record
	if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal chat log: %v", err)
	}

	return record
}
