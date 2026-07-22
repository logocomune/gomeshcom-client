package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/logocomune/gomeshcom-client/internal/callsign"
)

const Prefix = "GOMESHCOM"

type Config struct {
	conf.Version
	HTTPAddr         string `conf:"default:127.0.0.1:8080,help:HTTP listen address"`
	UDPListenAddr    string `conf:"default:0.0.0.0:1799,help:MeshCom UDP listen address"`
	NodeAddr         string `conf:"help:MeshCom node UDP address (auto-detected from incoming UDP traffic when empty)"`
	MyCall           string `conf:"default:QQ0XX-1,help:local callsign"`
	DataDir          string `conf:"default:./data,help:runtime data directory"`
	MaxMessageLength int    `conf:"default:149,help:maximum outgoing UTF-8 message length"`
	DemoMode         bool   `conf:"default:false,help:demo mode — disables TX and locks the config API"`
	ReceiveLog       ReceiveLog
	ChatLog          ChatLog
	Stats            Stats
	Send             Send
	Forward          Forward
	Auth             Auth
	RequestLog       RequestLog
	Compression      Compression
	Storage          Storage
	LogLevel         string `conf:"default:info,help:log level: debug|info|warn|error"`
}

type ReceiveLog struct {
	Enabled       bool          `conf:"default:true,help:enable received UDP JSONL log"`
	Path          string        `conf:"default:./data/raw,help:received UDP JSONL log directory"`
	RetentionDays int           `conf:"default:365,help:number of daily received UDP log files to keep"`
	ReplayWindow  time.Duration `conf:"default:1h,help:time window of received UDP packets replayed on SSE connect"`
}

// Stats configures the hourly statistics aggregator.
type Stats struct {
	Enabled       bool   `conf:"default:true,help:enable hourly packet statistics collection"`
	Path          string `conf:"default:./data/stats/stats.json,help:path to the stats JSON file"`
	RetentionDays int    `conf:"default:30,help:number of days of hourly buckets to retain"`
}

type ChatLog struct {
	Path             string        `conf:"default:./data/chat,help:chat JSONL directory"`
	HistoryWindow    time.Duration `conf:"default:24h,help:default chat history window returned by /api/chat/{conversation}"`
	MaxHistoryWindow time.Duration `conf:"default:720h,help:maximum chat history window allowed via ?hours= API parameter"`
}

type Send struct {
	DedupTTL time.Duration `conf:"default:2s,help:LRU TTL window for duplicate outgoing messages (0 disables)"`
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

type RequestLog struct {
	Enabled bool `conf:"default:false,help:enable structured HTTP request logging"`
}

type Compression struct {
	Enabled     bool `conf:"default:true,help:enable HTTP gzip response compression"`
	MinimumSize int  `conf:"default:1024,help:minimum response body size in bytes before gzip is applied"`
}

type Storage struct {
	SQLitePath          string        `conf:"default:./data/gomeshcom.db,help:path to the SQLite database file"`
	PurgeInterval       time.Duration `conf:"default:4h,help:interval between SQLite purge runs"`
	ReceiveLogRetention time.Duration `conf:"default:720h,help:SQLite receive_log retention window"`
	PublicChatRetention time.Duration `conf:"default:720h,help:SQLite public chat retention window"`
	NodesRetention      time.Duration `conf:"default:168h,help:SQLite nodes retention window based on last seen time"`
	TelemetryRetention  time.Duration `conf:"default:720h,help:SQLite telemetry retention window"`
}

// Load parses configuration from built-in defaults, a TOML file, and environment
// variables. Precedence (low → high): built-in defaults < TOML file < env vars.
//
// The TOML file path is derived from the data directory (env or default); changing
// data_dir inside the TOML has no effect — use GOMESHCOM_DATA_DIR to relocate data.
// A missing TOML file is created with commented defaults and startup continues.
// An invalid TOML file causes Load to return an error.
func Load(build string) (Config, EnvOverrides, string, error) {
	cfg := Config{
		Version: conf.Version{
			Build: build,
			Desc:  "gomeshcom MeshCom client",
		},
	}

	// Step 1: parse env + built-in defaults via ardanlabs/conf.
	// This resolves DataDir so we can locate the TOML file.
	info, err := conf.ParseWithOptions(Prefix, &cfg, conf.WithStrictFlags())
	if err != nil {
		return Config{}, nil, info, err
	}

	// Step 2: detect which fields are explicitly set via GOMESHCOM_* env vars.
	env := DetectEnvOverrides()

	// Step 3: write default TOML if missing (using built-in defaults, not env values),
	// then load it. WriteDefaultToml is a no-op when the file already exists.
	tomlPath := DefaultTomlPath(cfg.DataDir)
	if writeErr := WriteDefaultToml(tomlPath, builtInDefaultConfig()); writeErr != nil {
		_ = writeErr // non-fatal; logger not yet configured
	}

	tf, err := loadTomlFile(tomlPath)
	if err != nil {
		return Config{}, nil, info, fmt.Errorf("config file: %w", err)
	}

	// Step 4: apply TOML values for fields not already set by env.
	mergeToml(&cfg, tf, env)

	cfg = normalize(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, nil, info, err
	}

	return cfg, env, info, nil
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

// builtInDefaultConfig returns a Config populated with the same defaults coded in
// the struct tags (conf:"default:..."). Used only to generate the initial TOML
// template so the file never captures env-override values.
func builtInDefaultConfig() Config {
	return Config{
		HTTPAddr:         "127.0.0.1:8080",
		UDPListenAddr:    "0.0.0.0:1799",
		NodeAddr:         "",
		MyCall:           "QQ0XX-1",
		DataDir:          "./data",
		MaxMessageLength: 149,
		LogLevel:         "info",
		ReceiveLog: ReceiveLog{
			Enabled:       true,
			Path:          "./data/raw",
			RetentionDays: 365,
			ReplayWindow:  time.Hour,
		},
		Stats: Stats{
			Enabled:       true,
			Path:          "./data/stats/stats.json",
			RetentionDays: 30,
		},
		ChatLog: ChatLog{
			Path:             "./data/chat",
			HistoryWindow:    24 * time.Hour,
			MaxHistoryWindow: 720 * time.Hour,
		},
		Send: Send{
			DedupTTL: 2 * time.Second,
		},
		Forward:     Forward{Targets: ""},
		Auth:        Auth{SessionTTL: 24 * time.Hour, CookieName: "meshcom_session"},
		RequestLog:  RequestLog{Enabled: false},
		Compression: Compression{Enabled: true, MinimumSize: 1024},
		Storage: Storage{
			SQLitePath:          "./data/gomeshcom.db",
			PurgeInterval:       4 * time.Hour,
			ReceiveLogRetention: 30 * 24 * time.Hour,
			PublicChatRetention: 30 * 24 * time.Hour,
			NodesRetention:      7 * 24 * time.Hour,
			TelemetryRetention:  30 * 24 * time.Hour,
		},
	}
}

func normalize(cfg Config) Config {
	cfg.MyCall = callsign.Normalize(cfg.MyCall)
	cfg.Storage = normalizeStorage(cfg.Storage)
	return cfg
}

func normalizeStorage(storage Storage) Storage {
	if storage.PurgeInterval != 0 || storage.ReceiveLogRetention != 0 || storage.PublicChatRetention != 0 || storage.NodesRetention != 0 {
		return storage
	}
	storage.PurgeInterval = 4 * time.Hour
	storage.ReceiveLogRetention = 30 * 24 * time.Hour
	storage.PublicChatRetention = 30 * 24 * time.Hour
	storage.NodesRetention = 7 * 24 * time.Hour
	return storage
}

func Validate(cfg Config) error {
	cfg = normalize(cfg)
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

	if !callsign.IsValid(cfg.MyCall) {
		return errors.New("my call must be 3-10 alphanumeric characters with an optional numeric SSID (e.g. IU5PMP-1)")
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

	if cfg.Storage.SQLitePath == "" {
		return errors.New("storage sqlite path is required")
	}
	if cfg.Storage.PurgeInterval <= 0 {
		return errors.New("storage purge interval must be greater than zero")
	}
	if cfg.Storage.ReceiveLogRetention < 0 {
		return errors.New("storage receive log retention must not be negative")
	}
	if cfg.Storage.PublicChatRetention < 0 {
		return errors.New("storage public chat retention must not be negative")
	}
	if cfg.Storage.NodesRetention < 0 {
		return errors.New("storage nodes retention must not be negative")
	}
	if cfg.Storage.TelemetryRetention < 0 {
		return errors.New("storage telemetry retention must not be negative")
	}

	return nil
}
