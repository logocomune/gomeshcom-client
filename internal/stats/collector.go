package stats

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
	"github.com/logocomune/gomeshcom-client/internal/positions"
)

// positionGetter abstracts positions.Store.Get for testing.
type positionGetter interface {
	Get(callsign string) (positions.Record, bool)
}

// myCallSource provides the live local callsign. *station.Identity satisfies
// this interface; a static wrapper may be used in tests.
type myCallSource interface {
	Current() string
}

// Collector subscribes to the event bus and records packet statistics.
type Collector struct {
	store    *Store
	pos      positionGetter
	identity myCallSource
}

// NewCollector builds a Collector. identity provides the live local callsign.
func NewCollector(store *Store, posStore positionGetter, identity myCallSource) *Collector {
	return &Collector{
		store:    store,
		pos:      posStore,
		identity: identity,
	}
}

// myCall returns the current local callsign, or "" if no identity is configured.
func (c *Collector) myCall() string {
	if c.identity == nil {
		return ""
	}
	return c.identity.Current()
}

// Run subscribes to bus and processes events until ctx is cancelled.
func (c *Collector) Run(ctx context.Context, bus *events.Bus) {
	sub := bus.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub:
			if !ok {
				return
			}
			c.handle(ev)
		}
	}
}

func (c *Collector) handle(ev events.Event) {
	switch ev.Type {
	case "packet.received":
		c.handlePacket(ev)
	case "packet.error":
		c.store.RecordPacket(KindError, time.Now().UTC(), nil)
	case "message.delivered":
		c.handleDelivered(ev)
	}
}

func (c *Collector) handlePacket(ev events.Event) {
	data, ok := ev.Data.(map[string]any)
	if !ok {
		return
	}
	receivedAtStr, _ := data["received_at"].(string)
	receivedAt := parseReceivedAt(receivedAtStr)

	packet := data["packet"]
	switch typed := packet.(type) {
	case meshcom.TextMessage:
		if chatlog.IsDM(typed.Destination) {
			c.store.RecordPacket(KindDM, receivedAt, nil)
			// Channel key for inbound DM: partner is the origin of src.
			partner := originCallsign(typed.Source)
			c.store.RecordChannel(dmKey(partner), receivedAt)
		} else {
			c.store.RecordPacket(KindPublic, receivedAt, nil)
			c.store.RecordChannel(publicKey(typed.Destination), receivedAt)
		}
	case meshcom.Position:
		var distPtr *float64
		if myCall := c.myCall(); myCall != "" {
			if own, ok := c.pos.Get(myCall); ok && own.Latitude != 0 && own.Longitude != 0 {
				d := HaversineKm(own.Latitude, own.Longitude, typed.Latitude, typed.Longitude)
				distPtr = &d
			}
		}
		c.store.RecordPacket(KindPosition, receivedAt, distPtr)
	case meshcom.Telemetry:
		c.store.RecordPacket(KindTelemetry, receivedAt, nil)
	default:
		slog.Debug("stats: unclassified packet type", "type", ev.Type)
	}
}

func (c *Collector) handleDelivered(ev events.Event) {
	record, ok := ev.Data.(chatlog.Record)
	if !ok {
		return
	}
	if chatlog.IsDM(record.Dst) {
		now := time.Now().UTC()
		c.store.RecordDMAck(now)
		// Channel key for delivered ack: partner is Dst.
		c.store.RecordChannel(dmKey(record.Dst), now)
	}
}

// ---- key helpers ------------------------------------------------------------

// publicKey returns the Channels map key for a public/broadcast destination.
func publicKey(dst string) string {
	if dst == "" || dst == "*" {
		return "broadcast"
	}
	return "ch:" + dst
}

// dmKey returns the Channels map key for a DM conversation partner.
func dmKey(callsign string) string {
	if callsign == "" {
		return ""
	}
	return "dm:" + strings.ToUpper(callsign)
}

// originCallsign returns the origin part of a MeshCom source path
// (everything before the first comma).
func originCallsign(src string) string {
	return strings.ToUpper(strings.SplitN(src, ",", 2)[0])
}

func parseReceivedAt(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}
