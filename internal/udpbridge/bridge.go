package udpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/chatlog"
	"github.com/logocomune/gomeshcom-udp/internal/events"
	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
	"github.com/logocomune/gomeshcom-udp/internal/positions"
	"github.com/logocomune/gomeshcom-udp/internal/receivelog"
)

type Bridge struct {
	listenAddr      string
	nodeAddr        string // explicit config; "" means auto-detect
	learnedNodeAddr atomic.Pointer[string]
	bus             *events.Bus
	logger          *receivelog.Logger
	chatLog         *chatlog.Logger
	positions       *positions.Store
}

func NewBridge(listenAddr string, nodeAddr string, bus *events.Bus, logger *receivelog.Logger, chatLog *chatlog.Logger, positionStore *positions.Store) *Bridge {
	return &Bridge{listenAddr: listenAddr, nodeAddr: nodeAddr, bus: bus, logger: logger, chatLog: chatLog, positions: positionStore}
}

func (b *Bridge) Listen(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", b.listenAddr)
	if err != nil {
		return fmt.Errorf("resolve udp listen addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buffer := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read udp: %w", err)
		}

		rawPacket := string(buffer[:n])
		b.handleDatagram(remoteAddr.String(), buffer[:n], rawPacket)
	}
}

func (b *Bridge) handleDatagram(remoteAddr string, data []byte, rawPacket string) {
	slog.Debug("udp datagram received", "remote_addr", remoteAddr, "bytes", len(data), "raw", rawPacket)

	packet, err := meshcom.ParsePacket(data)
	if err != nil {
		b.logReceivedDatagram(remoteAddr, len(data), rawPacket, "", err.Error())
		slog.Debug("udp datagram parse failed", "remote_addr", remoteAddr, "bytes", len(data), "error", err)
		b.bus.Publish(events.Event{Type: "packet.error", Data: err.Error()})
		return
	}

	// Learn the node address from successfully parsed packets when not explicitly configured.
	if b.nodeAddr == "" {
		b.learnedNodeAddr.Store(&remoteAddr)
		slog.Debug("node addr learned from incoming packet", "remote_addr", remoteAddr)
	}

	b.logReceivedDatagram(remoteAddr, len(data), rawPacket, string(packet.Type), "")
	slog.Debug("udp packet parsed", "remote_addr", remoteAddr, "packet_type", packet.Type, "unknown_fields", len(packet.Unknown))

	switch typed := packet.Packet.(type) {
	case meshcom.Position:
		b.updatePositionStore(typed)
	case meshcom.TextMessage:
		b.logChatMessage(typed)
		b.touchPositionFreshness(typed.Source, typed.RSSI, typed.SNR)
	case meshcom.Telemetry:
		b.touchPositionFreshness(typed.Source, typed.RSSI, typed.SNR)
	}

	b.bus.Publish(events.Event{
		Type: "packet.received",
		Data: map[string]any{
			"remote_addr": remoteAddr,
			"packet":      packet.Packet,
		},
	})
}

// ErrNodeNotDetected is returned by SendText when NodeAddr is empty and no UDP
// packets have been received yet, so no address has been learned.
var ErrNodeNotDetected = errors.New("node address not configured and no UDP packets seen yet")

// effectiveNodeAddr returns the address to which outgoing UDP packets are sent.
// The explicitly configured nodeAddr always wins; if it is empty the last address
// learned from a successfully parsed incoming packet is used instead.
func (b *Bridge) effectiveNodeAddr() (string, error) {
	if b.nodeAddr != "" {
		return b.nodeAddr, nil
	}
	if p := b.learnedNodeAddr.Load(); p != nil && *p != "" {
		return *p, nil
	}
	return "", ErrNodeNotDetected
}

func (b *Bridge) updatePositionStore(position meshcom.Position) {
	if b.positions == nil {
		return
	}
	if b.positions.Update(position, time.Now().UTC()) {
		slog.Debug("position store updated", "source", position.Source)
	}
}

func (b *Bridge) touchPositionFreshness(src string, rssi, snr int) {
	if b.positions == nil {
		return
	}
	if b.positions.TouchFromPacket(src, rssi, snr, time.Now().UTC()) {
		slog.Debug("position freshness touched", "source", src)
	}
}

func (b *Bridge) logChatMessage(msg meshcom.TextMessage) {
	if b.chatLog == nil {
		return
	}
	if err := b.chatLog.Append(msg, time.Now().UTC()); err != nil {
		slog.Error("chat log write failed", "error", err)
	}
}

func (b *Bridge) logReceivedDatagram(remoteAddr string, bytes int, raw string, packetType string, parseError string) {
	if b.logger == nil {
		return
	}

	err := b.logger.Append(receivelog.Record{
		ReceivedAt: time.Now().UTC(),
		RemoteAddr: remoteAddr,
		Bytes:      bytes,
		Raw:        raw,
		PacketType: packetType,
		ParseError: parseError,
	})
	if err != nil {
		slog.Error("receive log write failed", "error", err)
	}
}

func (b *Bridge) SendText(ctx context.Context, destination string, message string, maxLength int) error {
	outgoing, err := meshcom.NewOutgoingText(destination, message, maxLength)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(outgoing)
	if err != nil {
		return fmt.Errorf("encode outgoing message: %w", err)
	}

	nodeAddr, err := b.effectiveNodeAddr()
	if err != nil {
		return err
	}

	source := "config"
	if b.nodeAddr == "" {
		source = "learned"
	}
	slog.Info("udp send", "node", nodeAddr, "source", source, "payload", string(payload))

	addr, err := net.ResolveUDPAddr("udp", nodeAddr)
	if err != nil {
		return fmt.Errorf("resolve node addr: %w", err)
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", addr.String())
	if err != nil {
		return fmt.Errorf("dial udp node: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set udp deadline: %w", err)
		}
	} else {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return fmt.Errorf("set udp deadline: %w", err)
		}
	}

	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write udp node: %w", err)
	}

	return nil
}
