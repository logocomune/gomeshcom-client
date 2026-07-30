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

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/packetingest"
	"github.com/logocomune/gomeshcom-client/internal/positions"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
	"github.com/logocomune/gomeshcom-client/internal/telemetry"
	"github.com/logocomune/gomeshcom-client/internal/transport"
	"github.com/logocomune/gomeshcom-client/internal/udpforward"
)

// chatStatusTracker is satisfied by chatstatus.Store.
type chatStatusTracker interface {
	RecordIncoming(convID string, ts time.Time, msg string)
}

// myCallSource provides the live local callsign. *station.Identity satisfies
// this interface; a static wrapper may be used in tests.
type myCallSource interface {
	Current() string
}

type Bridge struct {
	listenAddr      string
	nodeAddr        string // explicit config; "" means auto-detect
	learnedNodeAddr atomic.Pointer[string]
	processor       *packetingest.Processor
	disableTx       bool
	forwarder       *udpforward.Forwarder
}

func NewBridge(listenAddr, nodeAddr string, bus *events.Bus, logger *receivelog.Logger, chatLog *chatlog.Logger, positionStore *positions.Store, disableTx bool, forwarder *udpforward.Forwarder, identity myCallSource, chatStatus chatStatusTracker) *Bridge {
	dependencies := packetingest.Dependencies{
		Bus:        bus,
		ChatStatus: chatStatus,
		Identity:   identity,
	}
	if logger != nil {
		dependencies.ReceiveLog = logger
	}
	if chatLog != nil {
		dependencies.ChatLog = chatLog
	}
	if positionStore != nil {
		dependencies.Positions = positionStore
	}
	return NewBridgeWithProcessor(
		listenAddr,
		nodeAddr,
		packetingest.NewProcessor(dependencies),
		disableTx,
		forwarder,
	)
}

func NewBridgeWithProcessor(listenAddr, nodeAddr string, processor *packetingest.Processor, disableTx bool, forwarder *udpforward.Forwarder) *Bridge {
	return &Bridge{
		listenAddr: listenAddr,
		nodeAddr:   nodeAddr,
		processor:  processor,
		disableTx:  disableTx,
		forwarder:  forwarder,
	}
}

func (b *Bridge) SetTelemetryStore(store *telemetry.Store) {
	if store == nil {
		b.processor.SetTelemetryStore(nil)
		return
	}
	b.processor.SetTelemetryStore(store)
}

func (b *Bridge) TransportStatus() transport.Status {
	return transport.Status{
		Mode:     "udp",
		State:    transport.StateConnected,
		Endpoint: b.listenAddr,
	}
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
		if b.forwarder != nil {
			b.forwarder.Forward(buffer[:n])
		}
		b.handleDatagram(remoteAddr.String(), buffer[:n], rawPacket)
	}
}

func (b *Bridge) handleDatagram(remoteAddr string, data []byte, rawPacket string) {
	err := b.processor.Process(packetingest.Source{
		Transport: "udp",
		Endpoint:  remoteAddr,
	}, data)
	if err != nil {
		return
	}

	// Learn the node address from successfully parsed packets when not explicitly configured.
	if b.nodeAddr == "" {
		b.learnedNodeAddr.Store(&remoteAddr)
		slog.Debug("node addr learned from incoming packet", "remote_addr", remoteAddr)
	}
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

func (b *Bridge) SendText(ctx context.Context, destination string, message string, maxLength int) error {
	outgoing, err := meshcom.NewOutgoingText(destination, message, maxLength)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(outgoing)
	if err != nil {
		return fmt.Errorf("encode outgoing message: %w", err)
	}

	if b.disableTx {
		slog.Warn("udp tx disabled (dry-run)", "destination", destination, "payload", string(payload))
		return nil
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
