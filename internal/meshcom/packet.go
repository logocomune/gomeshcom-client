package meshcom

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type PacketType string

const (
	PacketTypeMessage   PacketType = "msg"
	PacketTypePosition  PacketType = "pos"
	PacketTypeTelemetry PacketType = "tele"
)

type Envelope struct {
	Type    PacketType
	Packet  any
	Raw     json.RawMessage
	Unknown map[string]json.RawMessage
}

type TextMessage struct {
	Type        PacketType  `json:"type"`
	Source      string      `json:"src,omitempty"`
	Destination string      `json:"dst,omitempty"`
	Message     string      `json:"msg"`
	MessageID   string      `json:"msg_id,omitempty"`
	SourceType  string      `json:"src_type,omitempty"`
	Firmware    StringValue `json:"firmware,omitempty"`
	FWSub       StringValue `json:"fw_sub,omitempty"`
	RSSI        int         `json:"rssi,omitempty"`
	SNR         int         `json:"snr,omitempty"`
}

type Position struct {
	Type            PacketType  `json:"type"`
	Source          string      `json:"src,omitempty"`
	Message         string      `json:"msg,omitempty"`
	MessageID       string      `json:"msg_id,omitempty"`
	SourceType      string      `json:"src_type,omitempty"`
	Latitude        float64     `json:"lat,omitempty"`
	LatitudeDir     string      `json:"lat_dir,omitempty"`
	Longitude       float64     `json:"long,omitempty"`
	LongitudeDir    string      `json:"long_dir,omitempty"`
	APRSSymbol      string      `json:"aprs_symbol,omitempty"`
	APRSSymbolGroup string      `json:"aprs_symbol_group,omitempty"`
	HardwareID      StringValue `json:"hw_id,omitempty"`
	Altitude        int         `json:"alt,omitempty"`
	Battery         int         `json:"batt,omitempty"`
	Firmware        StringValue `json:"firmware,omitempty"`
	FWSub           StringValue `json:"fw_sub,omitempty"`
	RSSI            int         `json:"rssi,omitempty"`
	SNR             int         `json:"snr,omitempty"`
}

type Telemetry struct {
	Type       PacketType `json:"type"`
	Source     string     `json:"src,omitempty"`
	SourceType string     `json:"src_type,omitempty"`
	RSSI       int        `json:"rssi,omitempty"`
	SNR        int        `json:"snr,omitempty"`
	Temp1      float64    `json:"temp1,omitempty"`
	Temp2      float64    `json:"temp2,omitempty"`
	Humidity   float64    `json:"hum,omitempty"`
	Battery    int        `json:"batt,omitempty"`
	QFE        float64    `json:"qfe,omitempty"`
	QNH        float64    `json:"qnh,omitempty"`
	Gas        float64    `json:"gas,omitempty"`
	CO2        float64    `json:"co2,omitempty"`
}

type OutgoingText struct {
	Type        PacketType `json:"type"`
	Destination string     `json:"dst"`
	Message     string     `json:"msg"`
}

type StringValue string

func (s *StringValue) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = StringValue(text)
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		*s = StringValue(number.String())
		return nil
	}

	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		*s = StringValue(strconv.FormatBool(boolean))
		return nil
	}

	if string(data) == "null" {
		*s = ""
		return nil
	}

	return fmt.Errorf("decode string value: %s", string(data))
}

func ParsePacket(data []byte) (Envelope, error) {
	raw := json.RawMessage(append([]byte(nil), data...))

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Envelope{}, fmt.Errorf("decode packet: %w", err)
	}

	packetType, err := readPacketType(fields)
	if err != nil {
		return Envelope{}, err
	}

	envelope := Envelope{
		Type:    packetType,
		Raw:     raw,
		Unknown: unknownFields(fields, knownFields(packetType)),
	}

	switch packetType {
	case PacketTypeMessage:
		var packet TextMessage
		if err := json.Unmarshal(data, &packet); err != nil {
			return Envelope{}, fmt.Errorf("decode text message: %w", err)
		}
		packet.Type = packetType
		envelope.Packet = packet
	case PacketTypePosition:
		var packet Position
		if err := json.Unmarshal(data, &packet); err != nil {
			return Envelope{}, fmt.Errorf("decode position: %w", err)
		}
		packet.Type = packetType
		envelope.Packet = packet
	case PacketTypeTelemetry:
		var packet Telemetry
		if err := json.Unmarshal(data, &packet); err != nil {
			return Envelope{}, fmt.Errorf("decode telemetry: %w", err)
		}
		packet.Type = packetType
		envelope.Packet = packet
	default:
		return Envelope{}, fmt.Errorf("unsupported packet type %q", packetType)
	}

	return envelope, nil
}

func NewOutgoingText(destination string, message string, maxLength int) (OutgoingText, error) {
	if err := ValidateOutgoingText(destination, message, maxLength); err != nil {
		return OutgoingText{}, err
	}

	return OutgoingText{
		Type:        PacketTypeMessage,
		Destination: destination,
		Message:     message,
	}, nil
}

func ValidateOutgoingText(destination string, message string, maxLength int) error {
	if strings.TrimSpace(destination) == "" {
		return errors.New("destination is required")
	}

	if message == "" {
		return errors.New("message is required")
	}

	if !utf8.ValidString(message) {
		return errors.New("message must be valid utf-8")
	}

	if utf8.RuneCountInString(message) > maxLength {
		return fmt.Errorf("message length exceeds %d characters", maxLength)
	}

	return nil
}

func readPacketType(fields map[string]json.RawMessage) (PacketType, error) {
	rawType, ok := fields["type"]
	if !ok {
		return "", errors.New("packet type is required")
	}

	var packetType PacketType
	if err := json.Unmarshal(rawType, &packetType); err != nil {
		return "", fmt.Errorf("decode packet type: %w", err)
	}

	if packetType == "" {
		return "", errors.New("packet type is required")
	}

	return packetType, nil
}

func unknownFields(fields map[string]json.RawMessage, known map[string]struct{}) map[string]json.RawMessage {
	unknown := make(map[string]json.RawMessage)
	for name, value := range fields {
		if _, ok := known[name]; !ok {
			unknown[name] = append(json.RawMessage(nil), value...)
		}
	}
	return unknown
}

// SplitSourcePath splits a comma-separated MeshCom source chain ("ORIGIN,R1,R2")
// into origin callsign and relay list. Empty or whitespace-only parts are dropped.
func SplitSourcePath(source string) (origin string, via []string) {
	parts := strings.Split(source, ",")
	if len(parts) == 0 {
		return "", nil
	}
	origin = strings.TrimSpace(parts[0])
	via = make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		relay := strings.TrimSpace(part)
		if relay != "" {
			via = append(via, relay)
		}
	}
	return origin, via
}

func knownFields(packetType PacketType) map[string]struct{} {
	names := []string{"type", "src_type", "src", "dst", "msg", "msg_id", "firmware", "fw_sub", "rssi", "snr"}

	switch packetType {
	case PacketTypePosition:
		names = append(names, "lat", "lat_dir", "long", "long_dir", "aprs_symbol", "aprs_symbol_group", "hw_id", "alt", "batt")
	case PacketTypeTelemetry:
		names = append(names, "temp1", "temp2", "hum", "batt", "qfe", "qnh", "gas", "co2")
	}

	known := make(map[string]struct{}, len(names))
	for _, name := range names {
		known[name] = struct{}{}
	}
	return known
}
