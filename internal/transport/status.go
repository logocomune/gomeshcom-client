package transport

import (
	"errors"
	"time"
)

var ErrUnavailable = errors.New("transport unavailable")

type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateDegraded     State = "degraded"
	StateStopped      State = "stopped"
)

type Status struct {
	Mode        string     `json:"mode"`
	State       State      `json:"state"`
	Endpoint    string     `json:"endpoint"`
	LastError   string     `json:"last_error,omitempty"`
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	RetryCount  uint64     `json:"retry_count"`
}

type StatusProvider interface {
	TransportStatus() Status
}
