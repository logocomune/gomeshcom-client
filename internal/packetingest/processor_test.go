package packetingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
)

type fakeReceiveLogger struct {
	records []receivelog.Record
	err     error
}

func (l *fakeReceiveLogger) Append(record receivelog.Record) error {
	l.records = append(l.records, record)
	return l.err
}

type fakeChatLogger struct {
	messages []meshcom.TextMessage
	err      error
}

func (l *fakeChatLogger) Append(message meshcom.TextMessage, _ time.Time) error {
	l.messages = append(l.messages, message)
	return l.err
}

type positionTouch struct {
	source string
	rssi   *int
	snr    *int
}

type fakePositionStore struct {
	positions   []meshcom.Position
	touches     []positionTouch
	updateReply bool
	touchReply  bool
}

func (s *fakePositionStore) Update(position meshcom.Position, _ time.Time) bool {
	s.positions = append(s.positions, position)
	return s.updateReply
}

func (s *fakePositionStore) TouchFromPacket(source string, rssi, snr *int, _ time.Time) bool {
	s.touches = append(s.touches, positionTouch{source: source, rssi: rssi, snr: snr})
	return s.touchReply
}

type fakeTelemetryStore struct {
	packets []meshcom.Telemetry
	err     error
}

func (s *fakeTelemetryStore) Append(_ context.Context, packet meshcom.Telemetry, _ time.Time) error {
	s.packets = append(s.packets, packet)
	return s.err
}

type incomingStatus struct {
	conversationID string
	message        string
}

type fakeChatStatus struct {
	records []incomingStatus
}

func (s *fakeChatStatus) RecordIncoming(conversationID string, _ time.Time, message string) {
	s.records = append(s.records, incomingStatus{
		conversationID: conversationID,
		message:        message,
	})
}

type staticIdentity string

func (i staticIdentity) Current() string {
	return string(i)
}

func TestProcessDispatchesValidPackets(t *testing.T) {
	tests := []struct {
		name              string
		raw               string
		wantType          string
		wantChat          int
		wantPositions     int
		wantTouches       int
		wantTelemetry     int
		wantStatusRecords int
	}{
		{
			name:              "text message",
			raw:               `{"src_type":"lora","type":"msg","src":"QQ1PEER-2","dst":"QQ1OWN-1","msg":"hello","rssi":-100,"snr":2}`,
			wantType:          "msg",
			wantChat:          1,
			wantTouches:       1,
			wantStatusRecords: 1,
		},
		{
			name:          "position",
			raw:           `{"src_type":"lora","type":"pos","src":"QQ1PEER-2","lat":43.1,"long":10.2}`,
			wantType:      "pos",
			wantPositions: 1,
		},
		{
			name:          "telemetry",
			raw:           `{"src_type":"lora","type":"tele","src":"QQ1PEER-2","temp1":21.5,"rssi":-101,"snr":3}`,
			wantType:      "tele",
			wantTouches:   1,
			wantTelemetry: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := events.NewBus()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			subscriber := bus.Subscribe(ctx)

			receiveLog := &fakeReceiveLogger{}
			chatLog := &fakeChatLogger{}
			positions := &fakePositionStore{updateReply: true, touchReply: true}
			telemetry := &fakeTelemetryStore{}
			status := &fakeChatStatus{}
			processor := NewProcessor(Dependencies{
				Bus:        bus,
				ReceiveLog: receiveLog,
				ChatLog:    chatLog,
				ChatStatus: status,
				Identity:   staticIdentity("QQ1OWN-1"),
				Positions:  positions,
				Telemetry:  telemetry,
			})

			source := Source{Transport: "serial", Endpoint: "/dev/ttyUSB0"}
			if err := processor.Process(source, []byte(tt.raw)); err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if len(receiveLog.records) != 1 {
				t.Fatalf("receive records = %d, want 1", len(receiveLog.records))
			}
			record := receiveLog.records[0]
			if record.RemoteAddr != source.Endpoint || record.Raw != tt.raw || record.PacketType != tt.wantType || record.ParseError != "" {
				t.Fatalf("receive record = %+v", record)
			}
			if len(chatLog.messages) != tt.wantChat {
				t.Fatalf("chat messages = %d, want %d", len(chatLog.messages), tt.wantChat)
			}
			if len(positions.positions) != tt.wantPositions {
				t.Fatalf("positions = %d, want %d", len(positions.positions), tt.wantPositions)
			}
			if len(positions.touches) != tt.wantTouches {
				t.Fatalf("position touches = %d, want %d", len(positions.touches), tt.wantTouches)
			}
			if len(telemetry.packets) != tt.wantTelemetry {
				t.Fatalf("telemetry packets = %d, want %d", len(telemetry.packets), tt.wantTelemetry)
			}
			if len(status.records) != tt.wantStatusRecords {
				t.Fatalf("status records = %+v, want %d", status.records, tt.wantStatusRecords)
			}

			event := readProcessorEvent(t, subscriber)
			if event.Type != "packet.received" {
				t.Fatalf("event type = %q, want packet.received", event.Type)
			}
			payload, ok := event.Data.(map[string]any)
			if !ok {
				t.Fatalf("event data type = %T, want map", event.Data)
			}
			if payload["transport"] != source.Transport ||
				payload["endpoint"] != source.Endpoint ||
				payload["remote_addr"] != source.Endpoint {
				t.Fatalf("event source metadata = %+v", payload)
			}
			if _, err := time.Parse(time.RFC3339Nano, payload["received_at"].(string)); err != nil {
				t.Fatalf("event received_at: %v", err)
			}
		})
	}
}

func TestProcessPublishesParseError(t *testing.T) {
	bus := events.NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscriber := bus.Subscribe(ctx)
	receiveLog := &fakeReceiveLogger{}
	processor := NewProcessor(Dependencies{Bus: bus, ReceiveLog: receiveLog})

	err := processor.Process(Source{Transport: "udp", Endpoint: "127.0.0.1:1799"}, []byte(`{`))
	if err == nil {
		t.Fatal("Process() error = nil")
	}
	if len(receiveLog.records) != 1 || receiveLog.records[0].ParseError == "" {
		t.Fatalf("receive records = %+v, want parse error", receiveLog.records)
	}
	event := readProcessorEvent(t, subscriber)
	if event.Type != "packet.error" {
		t.Fatalf("event type = %q, want packet.error", event.Type)
	}
	if event.Data != err.Error() {
		t.Fatalf("event data = %#v, want %q", event.Data, err.Error())
	}
}

func TestProcessFiltersChatStatus(t *testing.T) {
	tests := []struct {
		name     string
		identity Identity
		raw      string
		want     []incomingStatus
	}{
		{
			name:     "own device echo",
			identity: staticIdentity("QQ1OWN-1"),
			raw:      `{"type":"msg","src":"QQ1OWN-2","dst":"QQ1PEER-2","msg":"hello{123"}`,
		},
		{
			name:     "ack",
			identity: staticIdentity("QQ1OWN-1"),
			raw:      `{"type":"msg","src":"QQ1PEER-2","dst":"QQ1OWN-1","msg":"QQ1OWN-1:ack123"}`,
		},
		{
			name:     "unrelated direct message",
			identity: staticIdentity("QQ1OWN-1"),
			raw:      `{"type":"msg","src":"QQ1AAA-1","dst":"QQ1BBB-2","msg":"private"}`,
		},
		{
			name: "broadcast without identity",
			raw:  `{"type":"msg","src":"QQ1PEER-2","dst":"*","msg":"public"}`,
			want: []incomingStatus{{conversationID: "P_broadcast", message: "public"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &fakeChatStatus{}
			processor := NewProcessor(Dependencies{
				ChatLog:    &fakeChatLogger{},
				ChatStatus: status,
				Identity:   tt.identity,
			})

			if err := processor.Process(Source{Transport: "udp", Endpoint: "127.0.0.1:1799"}, []byte(tt.raw)); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if len(status.records) != len(tt.want) {
				t.Fatalf("status records = %+v, want %+v", status.records, tt.want)
			}
			for i := range tt.want {
				if status.records[i] != tt.want[i] {
					t.Fatalf("status record %d = %+v, want %+v", i, status.records[i], tt.want[i])
				}
			}
		})
	}
}

func TestProcessToleratesOptionalStoresAndWriteErrors(t *testing.T) {
	processor := NewProcessor(Dependencies{})
	valid := []byte(`{"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"hello"}`)
	if err := processor.Process(Source{Transport: "serial", Endpoint: "COM3"}, valid); err != nil {
		t.Fatalf("Process() with optional stores error = %v", err)
	}

	storeError := errors.New("store unavailable")
	processor = NewProcessor(Dependencies{
		ReceiveLog: &fakeReceiveLogger{err: storeError},
		ChatLog:    &fakeChatLogger{err: storeError},
		Positions:  &fakePositionStore{},
		Telemetry:  &fakeTelemetryStore{err: storeError},
	})
	processor.SetTelemetryStore(&fakeTelemetryStore{err: storeError})

	inputs := []string{
		`{"type":"msg","src":"QQ1AAA-1","dst":"*","msg":"hello"}`,
		`{"type":"pos","src":"QQ1AAA-1","lat":43.1,"long":10.2}`,
		`{"type":"tele","src":"QQ1AAA-1","temp1":20}`,
	}
	for _, input := range inputs {
		if err := processor.Process(Source{Transport: "serial", Endpoint: "COM3"}, []byte(input)); err != nil {
			t.Fatalf("Process(%s) error = %v", input, err)
		}
	}
}

func readProcessorEvent(t *testing.T, subscriber <-chan events.Event) events.Event {
	t.Helper()
	select {
	case event := <-subscriber:
		return event
	case <-time.After(time.Second):
		t.Fatal("event timeout")
		return events.Event{}
	}
}
