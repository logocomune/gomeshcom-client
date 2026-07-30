package packetingest

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
)

type Source struct {
	Transport string
	Endpoint  string
}

type ChatStatusTracker interface {
	RecordIncoming(conversationID string, receivedAt time.Time, message string)
}

type Identity interface {
	Current() string
}

type ReceiveLogger interface {
	Append(record receivelog.Record) error
}

type ChatLogger interface {
	Append(message meshcom.TextMessage, receivedAt time.Time) error
}

type PositionStore interface {
	Update(position meshcom.Position, receivedAt time.Time) bool
	TouchFromPacket(source string, rssi, snr *int, receivedAt time.Time) bool
}

type TelemetryStore interface {
	Append(ctx context.Context, packet meshcom.Telemetry, receivedAt time.Time) error
}

type Dependencies struct {
	Bus        *events.Bus
	ReceiveLog ReceiveLogger
	ChatLog    ChatLogger
	ChatStatus ChatStatusTracker
	Identity   Identity
	Positions  PositionStore
	Telemetry  TelemetryStore
}

type Processor struct {
	bus        *events.Bus
	receiveLog ReceiveLogger
	chatLog    ChatLogger
	chatStatus ChatStatusTracker
	identity   Identity
	positions  PositionStore
	telemetry  TelemetryStore
}

func NewProcessor(dependencies Dependencies) *Processor {
	return &Processor{
		bus:        dependencies.Bus,
		receiveLog: dependencies.ReceiveLog,
		chatLog:    dependencies.ChatLog,
		chatStatus: dependencies.ChatStatus,
		identity:   dependencies.Identity,
		positions:  dependencies.Positions,
		telemetry:  dependencies.Telemetry,
	}
}

func (p *Processor) SetTelemetryStore(store TelemetryStore) {
	p.telemetry = store
}

func (p *Processor) Process(source Source, data []byte) error {
	receivedAt := time.Now().UTC()
	slog.Debug(
		"transport packet received",
		"transport", source.Transport,
		"endpoint", source.Endpoint,
		"bytes", len(data),
	)

	packet, err := meshcom.ParsePacket(data)
	if err != nil {
		p.logReceivedPacket(source.Endpoint, data, "", err.Error(), receivedAt)
		slog.Debug(
			"transport packet parse failed",
			"transport", source.Transport,
			"endpoint", source.Endpoint,
			"bytes", len(data),
			"error", err,
		)
		p.publish(events.Event{Type: "packet.error", Data: err.Error()})
		return err
	}

	p.logReceivedPacket(source.Endpoint, data, string(packet.Type), "", receivedAt)
	slog.Debug(
		"transport packet parsed",
		"transport", source.Transport,
		"endpoint", source.Endpoint,
		"packet_type", packet.Type,
		"unknown_fields", len(packet.Unknown),
	)
	p.storePacket(packet.Packet, receivedAt)
	p.publishReceived(source, packet.Packet, receivedAt)
	return nil
}

func (p *Processor) storePacket(packet any, receivedAt time.Time) {
	switch typed := packet.(type) {
	case meshcom.Position:
		p.updatePositionStore(typed, receivedAt)
	case meshcom.TextMessage:
		p.logChatMessage(typed, receivedAt)
		p.touchPositionFreshness(typed.Source, typed.RSSI, typed.SNR, receivedAt)
	case meshcom.Telemetry:
		p.touchPositionFreshness(typed.Source, typed.RSSI, typed.SNR, receivedAt)
		p.logTelemetry(typed, receivedAt)
	}
}

func (p *Processor) updatePositionStore(position meshcom.Position, receivedAt time.Time) {
	if p.positions == nil {
		return
	}
	if p.positions.Update(position, receivedAt) {
		slog.Debug("position store updated", "source", position.Source)
	}
}

func (p *Processor) touchPositionFreshness(source string, rssi, snr *int, receivedAt time.Time) {
	if p.positions == nil {
		return
	}
	if p.positions.TouchFromPacket(source, rssi, snr, receivedAt) {
		slog.Debug("position freshness touched", "source", source)
	}
}

func (p *Processor) logTelemetry(packet meshcom.Telemetry, receivedAt time.Time) {
	if p.telemetry == nil {
		return
	}
	if err := p.telemetry.Append(context.Background(), packet, receivedAt); err != nil {
		slog.Error("telemetry write failed", "error", err)
	}
}

func (p *Processor) logChatMessage(message meshcom.TextMessage, receivedAt time.Time) {
	if p.chatLog == nil {
		return
	}
	if err := p.chatLog.Append(message, receivedAt); err != nil {
		slog.Error("chat log write failed", "error", err)
	}
	p.recordIncomingStatus(message, receivedAt)
}

func (p *Processor) recordIncomingStatus(message meshcom.TextMessage, receivedAt time.Time) {
	if p.chatStatus == nil || meshcom.IsAckOrReject(message.Message) {
		return
	}
	myCall := p.myCall()
	origin := strings.ToUpper(strings.SplitN(message.Source, ",", 2)[0])
	if chatlog.BaseCall(origin) == chatlog.BaseCall(myCall) {
		return
	}
	statusKey := chatlog.StatusKey(message.Source, message.Destination, myCall)
	if statusKey != "" {
		p.chatStatus.RecordIncoming(statusKey, receivedAt, message.Message)
	}
}

func (p *Processor) myCall() string {
	if p.identity == nil {
		return ""
	}
	return p.identity.Current()
}

func (p *Processor) logReceivedPacket(endpoint string, data []byte, packetType string, parseError string, receivedAt time.Time) {
	if p.receiveLog == nil {
		return
	}
	err := p.receiveLog.Append(receivelog.Record{
		ReceivedAt: receivedAt,
		RemoteAddr: endpoint,
		Bytes:      len(data),
		Raw:        string(data),
		PacketType: packetType,
		ParseError: parseError,
	})
	if err != nil {
		slog.Error("receive log write failed", "error", err)
	}
}

func (p *Processor) publishReceived(source Source, packet any, receivedAt time.Time) {
	p.publish(events.Event{
		Type: "packet.received",
		Data: map[string]any{
			"transport":   source.Transport,
			"endpoint":    source.Endpoint,
			"remote_addr": source.Endpoint,
			"packet":      packet,
			"received_at": receivedAt.Format(time.RFC3339Nano),
		},
	})
}

func (p *Processor) publish(event events.Event) {
	if p.bus != nil {
		p.bus.Publish(event)
	}
}
