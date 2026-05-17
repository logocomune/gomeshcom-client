package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/ardanlabs/conf/v3"
)

const Prefix = "GOMESHCOM"

var callsignPattern = regexp.MustCompile(`^[A-Z0-9]{3,10}-[0-9]{1,2}$`)

type Config struct {
	conf.Version
	HTTPAddr         string        `conf:"default:127.0.0.1:8080,help:HTTP listen address"`
	UDPListenAddr    string        `conf:"default:0.0.0.0:1799,help:MeshCom UDP listen address"`
	NodeAddr         string        `conf:"help:MeshCom node UDP address (auto-detected from incoming UDP traffic when empty)"`
	MyCall           string        `conf:"default:XX0XX-1,help:local callsign"`
	DataDir          string        `conf:"default:./data,help:runtime data directory"`
	SendDelay        time.Duration `conf:"default:40s,help:minimum delay between outgoing UDP messages"`
	MaxMessageLength int           `conf:"default:149,help:maximum outgoing UTF-8 message length"`
	ReceiveLog       ReceiveLog
	ChatLog          ChatLog
	Send             Send
	Forward          Forward
	Auth             Auth
	LogLevel         string `conf:"default:info,help:log level: debug|info|warn|error"`
}

type ReceiveLog struct {
	Enabled       bool          `conf:"default:true,help:enable received UDP JSONL log"`
	Path          string        `conf:"default:./data/raw,help:received UDP JSONL log directory"`
	RetentionDays int           `conf:"default:365,help:number of daily received UDP log files to keep"`
	ReplayWindow  time.Duration `conf:"default:1h,help:time window of received UDP packets replayed on SSE connect"`
}

type ChatLog struct {
	Path             string        `conf:"default:./data/chat,help:chat JSONL directory"`
	HistoryWindow    time.Duration `conf:"default:24h,help:default chat history window returned by /api/chat/{conversation}"`
	MaxHistoryWindow time.Duration `conf:"default:720h,help:maximum chat history window allowed via ?hours= API parameter"`
}

type Send struct {
	DedupTTL  time.Duration `conf:"default:2s,help:LRU TTL window for duplicate outgoing messages (0 disables)"`
	DisableTx bool          `conf:"default:false,help:dry-run mode — log outgoing UDP messages as warnings without transmitting"`
}

type Forward struct {
	Targets string `conf:"help:comma-separated host:port list; received UDP datagrams are mirrored unmodified to each target"`
}

type Auth struct {
	Username   string        `conf:"help:optional HTTP auth username"`
	Password   string        `conf:"help:optional HTTP auth password"`
	SessionTTL time.Duration `conf:"default:24h,help:HTTP auth session TTL"`
	CookieName string        `conf:"default:meshcom_session,help:HTTP auth session cookie name"`
}

func Load(build string) (Config, string, error) {
	cfg := Config{
		Version: conf.Version{
			Build: build,
			Desc:  "gomeshcom MeshCom client",
		},
	}

	info, err := conf.ParseWithOptions(Prefix, &cfg, conf.WithStrictFlags())
	if err != nil {
		return Config{}, info, err
	}

	cfg = normalize(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, info, err
	}

	return cfg, info, nil
}

// ParseForwardTargets splits the CSV forward-targets string, trims whitespace,
// deduplicates, and validates each entry as a resolvable UDP address.
func ParseForwardTargets(csv string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	for _, raw := range strings.Split(csv, ",") {
		t := strings.TrimSpace(raw)
		if t == "" || seen[t] {
			continue
		}
		if _, err := net.ResolveUDPAddr("udp", t); err != nil {
			return nil, fmt.Errorf("%q: %w", t, err)
		}
		seen[t] = true
		result = append(result, t)
	}
	return result, nil
}

func normalize(cfg Config) Config {
	cfg.MyCall = strings.ToUpper(cfg.MyCall)
	return cfg
}

func Validate(cfg Config) error {
	if _, err := net.ResolveTCPAddr("tcp", cfg.HTTPAddr); err != nil {
		return fmt.Errorf("http addr: %w", err)
	}

	if _, err := net.ResolveUDPAddr("udp", cfg.UDPListenAddr); err != nil {
		return fmt.Errorf("udp listen addr: %w", err)
	}

	if cfg.NodeAddr != "" {
		if _, err := net.ResolveUDPAddr("udp", cfg.NodeAddr); err != nil {
			return fmt.Errorf("node addr: %w", err)
		}
	}

	if cfg.MaxMessageLength <= 0 {
		return errors.New("max message length must be greater than zero")
	}

	if cfg.DataDir == "" {
		return errors.New("data dir is required")
	}

	if !callsignPattern.MatchString(cfg.MyCall) {
		return errors.New("my call must be an uppercase callsign with a numeric SSID")
	}

	if cfg.ReceiveLog.Enabled {
		if cfg.ReceiveLog.Path == "" {
			return errors.New("receive log path is required")
		}

		if cfg.ReceiveLog.RetentionDays < 0 {
			return errors.New("receive log retention days must not be negative")
		}

		if cfg.ReceiveLog.ReplayWindow < 0 {
			return errors.New("receive log replay window must not be negative")
		}
	}

	if cfg.ChatLog.Path == "" {
		return errors.New("chat log path is required")
	}

	if cfg.ChatLog.HistoryWindow <= 0 {
		return errors.New("chat log history window must be greater than zero")
	}

	if cfg.ChatLog.MaxHistoryWindow < cfg.ChatLog.HistoryWindow {
		return errors.New("chat log max history window must be >= history window")
	}

	if cfg.SendDelay < 0 {
		return errors.New("send delay must not be negative")
	}

	if cfg.Send.DedupTTL < 0 {
		return errors.New("send dedup TTL must not be negative")
	}

	if cfg.Forward.Targets != "" {
		if _, err := ParseForwardTargets(cfg.Forward.Targets); err != nil {
			return fmt.Errorf("forward targets: %w", err)
		}
	}

	authEnabled := cfg.Auth.Username != "" || cfg.Auth.Password != ""
	if authEnabled {
		if cfg.Auth.Username == "" || cfg.Auth.Password == "" {
			return errors.New("auth username and password must be set together")
		}
		if cfg.Auth.SessionTTL <= 0 {
			return errors.New("auth session TTL must be greater than zero")
		}
		if cfg.Auth.CookieName == "" {
			return errors.New("auth cookie name is required")
		}
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("log level must be debug, info, warn, or error")
	}

	return nil
}
