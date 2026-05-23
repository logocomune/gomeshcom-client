// iot-simulator emits MeshCom ExtUDP packets for local testing.
//
// Usage:
//
//	iot-simulator -my-call QQ5ABC-1 -target 127.0.0.1:1799
//	iot-simulator -my-call QQ5ABC-1 -target 127.0.0.1:1799 -listen-addr :1798 -interval 1m -pos2-interval 2m
//	iot-simulator -my-call QQ5ABC-1 -enable-pos1 -enable-pos2 -enable-dm -enable-broadcast -enable-chan2
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/logfmt"
	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

const (
	defaultLatitude  = 43.7303
	defaultLongitude = 10.3956
	defaultJitter    = 0.0005
	directSource     = "QQ1TST-1"
	repeaterSource   = "QQ1TST-2"
)

type config struct {
	listenAddr        string
	targetAddr        string
	myCall            string
	logLevel          string
	position1Interval time.Duration
	position2Interval time.Duration
	dmInterval        time.Duration
	enablePosition1   bool
	enablePosition2   bool
	enableDM          bool
	enableBroadcast   bool
	enableChannel2    bool
	ackDelay          time.Duration
	latitude          float64
	longitude         float64
	jitter            float64
	altitude          int
	battery           int
	hardwareID        string
	firmware          string
	seed              int64
}

type positionOptions struct {
	source     string
	latitude   float64
	longitude  float64
	jitter     float64
	altitude   int
	battery    int
	hardwareID string
	firmware   string
}

type messageOptions struct {
	myCall       string
	directSource string
	repeater     string
	ackDelay     time.Duration
}

type udpSender struct {
	conn   *net.UDPConn
	random *rand.Rand
	mu     sync.Mutex
}

func main() {
	cfg := parseFlags()
	configureLogger(cfg.logLevel)
	if err := run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func configureLogger(levelName string) {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	level, ok := levels[levelName]
	if !ok {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(logfmt.New(os.Stdout, level)))
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.listenAddr, "listen-addr", ":1798", "local UDP address for receiving node-bound datagrams")
	flag.StringVar(&cfg.targetAddr, "target", "127.0.0.1:1799", "destination UDP address for emitted packets")
	flag.StringVar(&cfg.myCall, "my-call", "", "local callsign used as DM destination (required)")
	flag.StringVar(&cfg.logLevel, "log-level", "info", "minimum log level (debug, info, warn, error)")
	flag.DurationVar(&cfg.position1Interval, "interval", time.Minute, "QQ1TST-1 position send interval")
	flag.DurationVar(&cfg.position2Interval, "pos2-interval", 2*time.Minute, "QQ1TST-2 position send interval")
	flag.DurationVar(&cfg.dmInterval, "dm-interval", time.Minute, "test DM send interval")
	flag.BoolVar(&cfg.enablePosition1, "enable-pos1", false, "enable QQ1TST-1 automatic position sends")
	flag.BoolVar(&cfg.enablePosition2, "enable-pos2", false, "enable QQ1TST-2 automatic position sends")
	flag.BoolVar(&cfg.enableDM, "enable-dm", false, "enable automatic DM sends to -my-call")
	flag.BoolVar(&cfg.enableBroadcast, "enable-broadcast", false, "enable automatic broadcast sends to *")
	flag.BoolVar(&cfg.enableChannel2, "enable-chan2", false, "enable automatic channel sends to 2")
	flag.DurationVar(&cfg.ackDelay, "ack-delay", 5*time.Second, "repeater ACK and mirror delay")
	flag.Float64Var(&cfg.latitude, "lat", defaultLatitude, "base latitude")
	flag.Float64Var(&cfg.longitude, "long", defaultLongitude, "base longitude")
	flag.Float64Var(&cfg.jitter, "jitter", defaultJitter, "maximum random coordinate offset in decimal degrees")
	flag.IntVar(&cfg.altitude, "alt", 12, "simulated altitude in meters")
	flag.IntVar(&cfg.battery, "batt", 95, "simulated battery percentage")
	flag.StringVar(&cfg.hardwareID, "hw-id", "IOT-SIM", "simulated hardware identifier")
	flag.StringVar(&cfg.firmware, "firmware", "sim", "simulated firmware version")
	flag.Int64Var(&cfg.seed, "seed", time.Now().UnixNano(), "random seed")
	flag.Parse()
	return cfg
}

func run(parent context.Context, cfg config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	listenAddr, err := net.ResolveUDPAddr("udp", cfg.listenAddr)
	if err != nil {
		return fmt.Errorf("resolve listen address %q: %w", cfg.listenAddr, err)
	}

	targetAddr, err := net.ResolveUDPAddr("udp", cfg.targetAddr)
	if err != nil {
		return fmt.Errorf("resolve target address %q: %w", cfg.targetAddr, err)
	}

	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen on %q: %w", cfg.listenAddr, err)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sender := &udpSender{
		conn:   conn,
		random: rand.New(rand.NewSource(cfg.seed)),
	}
	firstPosition := newPositionOptions(cfg, directSource)
	secondPosition := newPositionOptions(cfg, repeaterSource)
	messageOptions := messageOptions{
		myCall:       strings.ToUpper(strings.TrimSpace(cfg.myCall)),
		directSource: directSource,
		repeater:     repeaterSource,
		ackDelay:     cfg.ackDelay,
	}

	go receiveDatagrams(ctx, sender, targetAddr, messageOptions)

	slog.Info("iot-simulator listening", "listen", conn.LocalAddr(), "target", targetAddr, "my_call", messageOptions.myCall)
	autoSendsEnabled := cfg.enablePosition1 || cfg.enablePosition2 || cfg.enableDM || cfg.enableBroadcast || cfg.enableChannel2
	if !autoSendsEnabled {
		slog.Info("automatic sends disabled", "hint", "-enable-pos1/-enable-pos2/-enable-dm/-enable-broadcast/-enable-chan2")
		<-ctx.Done()
		return nil
	}

	dmCount := 1
	if cfg.enablePosition1 {
		if err := sender.sendPosition(targetAddr, firstPosition); err != nil {
			return err
		}
	}
	if cfg.enablePosition2 {
		if err := sender.sendPosition(targetAddr, secondPosition); err != nil {
			return err
		}
	}
	if cfg.enableDM {
		if err := sender.sendDirectMessage(targetAddr, messageOptions, dmCount); err != nil {
			return err
		}
	}
	if cfg.enableBroadcast {
		if err := sender.sendText(targetAddr, "lora", directSource, "*", fmt.Sprintf("Broadcast test %d", dmCount)); err != nil {
			return err
		}
	}
	if cfg.enableChannel2 {
		if err := sender.sendText(targetAddr, "lora", directSource, "2", fmt.Sprintf("Channel 2 test %d", dmCount)); err != nil {
			return err
		}
	}

	var position1Ticker <-chan time.Time
	if cfg.enablePosition1 {
		ticker := time.NewTicker(cfg.position1Interval)
		defer ticker.Stop()
		position1Ticker = ticker.C
	}

	var position2Ticker <-chan time.Time
	if cfg.enablePosition2 {
		ticker := time.NewTicker(cfg.position2Interval)
		defer ticker.Stop()
		position2Ticker = ticker.C
	}

	var dmTicker <-chan time.Time
	if cfg.enableDM || cfg.enableBroadcast || cfg.enableChannel2 {
		ticker := time.NewTicker(cfg.dmInterval)
		defer ticker.Stop()
		dmTicker = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-position1Ticker:
			if err := sender.sendPosition(targetAddr, firstPosition); err != nil {
				return err
			}
		case <-position2Ticker:
			if err := sender.sendPosition(targetAddr, secondPosition); err != nil {
				return err
			}
		case <-dmTicker:
			dmCount++
			if cfg.enableDM {
				if err := sender.sendDirectMessage(targetAddr, messageOptions, dmCount); err != nil {
					return err
				}
			}
			if cfg.enableBroadcast {
				if err := sender.sendText(targetAddr, "lora", directSource, "*", fmt.Sprintf("Broadcast test %d", dmCount)); err != nil {
					return err
				}
			}
			if cfg.enableChannel2 {
				if err := sender.sendText(targetAddr, "lora", directSource, "2", fmt.Sprintf("Channel 2 test %d", dmCount)); err != nil {
					return err
				}
			}
		}
	}
}

func newPositionOptions(cfg config, source string) positionOptions {
	return positionOptions{
		source:     source,
		latitude:   cfg.latitude,
		longitude:  cfg.longitude,
		jitter:     cfg.jitter,
		altitude:   cfg.altitude,
		battery:    cfg.battery,
		hardwareID: cfg.hardwareID,
		firmware:   cfg.firmware,
	}
}

func validateConfig(cfg config) error {
	if cfg.targetAddr == "" {
		return fmt.Errorf("-target is required")
	}
	if strings.TrimSpace(cfg.myCall) == "" {
		return fmt.Errorf("-my-call is required")
	}
	if cfg.enablePosition1 && cfg.position1Interval <= 0 {
		return fmt.Errorf("-interval must be greater than zero")
	}
	if cfg.enablePosition2 && cfg.position2Interval <= 0 {
		return fmt.Errorf("-pos2-interval must be greater than zero")
	}
	if (cfg.enableDM || cfg.enableBroadcast || cfg.enableChannel2) && cfg.dmInterval <= 0 {
		return fmt.Errorf("-dm-interval must be greater than zero")
	}
	if cfg.ackDelay < 0 {
		return fmt.Errorf("-ack-delay must be zero or greater")
	}
	if cfg.jitter < 0 {
		return fmt.Errorf("-jitter must be zero or greater")
	}
	return nil
}

func (s *udpSender) sendPosition(target *net.UDPAddr, options positionOptions) error {
	packet := s.newPositionPacket(options)
	return s.sendPacket(target, packet, func(written int) {
		slog.Info("sent position", "bytes", written, "target", target, "src", packet.Source, "lat", packet.Latitude, "long", packet.Longitude, "msg_id", packet.MessageID)
	})
}

func (s *udpSender) sendDirectMessage(target *net.UDPAddr, options messageOptions, count int) error {
	packet := s.newTextMessage("lora", directSource, options.myCall, fmt.Sprintf("DM test %d", count))
	return s.sendPacket(target, packet, func(written int) {
		slog.Info("sent DM", "bytes", written, "target", target, "src", packet.Source, "dst", packet.Destination, "msg", packet.Message)
	})
}

func (s *udpSender) sendText(target *net.UDPAddr, sourceType string, source string, destination string, message string) error {
	packet := s.newTextMessage(sourceType, source, destination, message)
	return s.sendPacket(target, packet, func(written int) {
		slog.Info("sent message", "bytes", written, "target", target, "src", packet.Source, "dst", packet.Destination, "msg", packet.Message)
	})
}

func (s *udpSender) sendPacket(target *net.UDPAddr, packet any, logSent func(written int)) error {
	data, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("encode packet: %w", err)
	}

	written, err := s.conn.WriteToUDP(data, target)
	if err != nil {
		return fmt.Errorf("send packet to %s: %w", target, err)
	}

	logSent(written)
	logPacket("TX", data)
	return nil
}

func receiveDatagrams(ctx context.Context, sender *udpSender, defaultTarget *net.UDPAddr, options messageOptions) {
	buf := make([]byte, 65535)
	for {
		if err := sender.conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			slog.Error("receive deadline failed", "error", err)
			return
		}

		n, remote, err := sender.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("receive failed", "error", err)
				return
			}
		}

		slog.Debug("received datagram", "bytes", n, "remote", remote)
		logPacket("RX", buf[:n])
		handleReceivedDatagram(ctx, sender, defaultTarget, append([]byte(nil), buf[:n]...), options)
	}
}

func handleReceivedDatagram(ctx context.Context, sender *udpSender, target *net.UDPAddr, data []byte, options messageOptions) {
	envelope, err := meshcom.ParsePacket(data)
	if err != nil {
		return
	}
	message, ok := envelope.Packet.(meshcom.TextMessage)
	if !ok {
		return
	}

	destination := strings.ToUpper(strings.TrimSpace(message.Destination))
	switch {
	case destination == options.directSource:
		respondToDirectMessage(sender, target, message, options)
	case destination == options.repeater:
		go respondAsRepeater(ctx, sender, target, message, options)
	case isPublicOrChannelDestination(destination):
		respondToBroadcast(sender, target, message, options)
	}
}

func isPublicOrChannelDestination(destination string) bool {
	if destination == "*" {
		return true
	}
	if destination == "" {
		return false
	}
	for _, char := range destination {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func respondToBroadcast(sender *udpSender, target *net.UDPAddr, received meshcom.TextMessage, options messageOptions) {
	destination := strings.ToUpper(strings.TrimSpace(received.Destination))
	if err := sender.sendText(target, "node", options.myCall, destination, received.Message); err != nil {
		slog.Error("broadcast reply failed", "dst", destination, "error", err)
	}
}

func respondToDirectMessage(sender *udpSender, target *net.UDPAddr, received meshcom.TextMessage, options messageOptions) {
	replyTo := replyCallsign(received, options.myCall)
	seq := sequenceID(received.Message)
	echoMessage := messageWithSequence(received.Message, seq)
	if err := sender.sendText(target, "node", replyTo, options.directSource, echoMessage); err != nil {
		slog.Error("echo failed", "dst", options.directSource, "error", err)
		return
	}
	if err := sender.sendText(target, "lora", options.directSource, replyTo, ackMessage(replyTo, seq)); err != nil {
		slog.Error("ack failed", "src", options.directSource, "error", err)
	}
}

func respondAsRepeater(ctx context.Context, sender *udpSender, target *net.UDPAddr, received meshcom.TextMessage, options messageOptions) {
	replyTo := replyCallsign(received, options.myCall)
	seq := sequenceID(received.Message)
	echoMessage := messageWithSequence(received.Message, seq)
	if err := sender.sendText(target, "node", replyTo, options.repeater, echoMessage); err != nil {
		slog.Error("echo failed", "dst", options.repeater, "error", err)
		return
	}

	timer := time.NewTimer(options.ackDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	if err := sender.sendText(target, "lora", options.repeater, replyTo, ackMessage(replyTo, seq)); err != nil {
		slog.Error("ack failed", "src", options.repeater, "error", err)
		return
	}
	if err := sender.sendText(target, "lora", options.repeater, replyTo, "MIRROR: "+cleanSequence(received.Message)); err != nil {
		slog.Error("mirror failed", "src", options.repeater, "error", err)
	}
}

func (s *udpSender) newPositionPacket(options positionOptions) meshcom.Position {
	s.mu.Lock()
	defer s.mu.Unlock()
	return meshcom.Position{
		Type:            meshcom.PacketTypePosition,
		SourceType:      "node",
		Source:          options.source,
		Message:         "",
		Latitude:        jitterCoordinate(s.random, options.latitude, options.jitter),
		LatitudeDir:     "N",
		Longitude:       jitterCoordinate(s.random, options.longitude, options.jitter),
		LongitudeDir:    "E",
		APRSSymbol:      "&",
		APRSSymbolGroup: "/",
		HardwareID:      meshcom.StringValue(options.hardwareID),
		MessageID:       newMessageID(s.random),
		Altitude:        options.altitude,
		Battery:         options.battery,
		Firmware:        meshcom.StringValue(options.firmware),
		FWSub:           "p",
		RSSI:            intPtr(0),
		SNR:             intPtr(0),
	}
}

func (s *udpSender) newTextMessage(sourceType string, source string, destination string, message string) meshcom.TextMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return meshcom.TextMessage{
		Type:        meshcom.PacketTypeMessage,
		SourceType:  sourceType,
		Source:      strings.ToUpper(source),
		Destination: strings.ToUpper(destination),
		Message:     message,
		MessageID:   newMessageID(s.random),
		Firmware:    meshcom.StringValue("sim"),
		FWSub:       "p",
		RSSI:        intPtr(-70),
		SNR:         intPtr(8),
	}
}

func logPacket(direction string, data []byte) {
	envelope, err := meshcom.ParsePacket(data)
	if err != nil {
		slog.Debug("packet raw", "dir", direction, "raw", string(data))
		return
	}

	source := "-"
	destination := "-"
	messageType := string(envelope.Type)
	switch packet := envelope.Packet.(type) {
	case meshcom.TextMessage:
		source = packet.Source
		destination = packet.Destination
	case meshcom.Position:
		source = packet.Source
	case meshcom.Telemetry:
		source = packet.Source
	}

	slog.Debug("packet", "dir", direction, "type", messageType, "src", source, "dst", destination)
}

func replyCallsign(message meshcom.TextMessage, fallback string) string {
	origin, _ := meshcom.SplitSourcePath(message.Source)
	origin = strings.ToUpper(strings.TrimSpace(origin))
	if origin != "" {
		return origin
	}
	return strings.ToUpper(strings.TrimSpace(fallback))
}

func intPtr(value int) *int { return &value }

func jitterCoordinate(random *rand.Rand, base float64, jitter float64) float64 {
	if jitter == 0 {
		return base
	}
	return base + (random.Float64()*2-1)*jitter
}

func newMessageID(random *rand.Rand) string {
	return fmt.Sprintf("%08X", random.Uint32())
}

func sequenceID(message string) string {
	match := trailingSequence(message)
	if match != "" {
		return match
	}
	return "123"
}

func trailingSequence(message string) string {
	start := strings.LastIndex(message, "{")
	if start < 0 || start == len(message)-1 {
		return ""
	}
	seq := strings.TrimSuffix(message[start+1:], "}")
	if seq == "" {
		return ""
	}
	for _, char := range seq {
		if char < '0' || char > '9' {
			return ""
		}
	}
	return seq
}

func messageWithSequence(message string, seq string) string {
	if trailingSequence(message) != "" {
		return message
	}
	return fmt.Sprintf("%s{%s", message, seq)
}

func cleanSequence(message string) string {
	if trailingSequence(message) == "" {
		return message
	}
	return strings.TrimSpace(message[:strings.LastIndex(message, "{")])
}

func ackMessage(myCall string, seq string) string {
	return fmt.Sprintf("%s :ack%s", strings.ToUpper(myCall), seq)
}
