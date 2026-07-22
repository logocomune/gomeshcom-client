package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// DefaultTomlPath returns the canonical path for the TOML configuration file.
// The path is always relative to dataDir and does not depend on TOML contents,
// which avoids a circular dependency when DataDir itself is set in the file.
func DefaultTomlPath(dataDir string) string {
	return filepath.Join(dataDir, "configs", "gomeshcomd.toml")
}

// -------- TOML wire types (pointer = optional / absent-from-file) --------

// tomlFile mirrors Config with pointer fields so absent keys decode to nil.
type tomlFile struct {
	HTTPAddr         *string          `toml:"http_addr"`
	UDPListenAddr    *string          `toml:"udp_listen_addr"`
	NodeAddr         *string          `toml:"node_addr"`
	MyCall           *string          `toml:"my_call"`
	DataDir          *string          `toml:"data_dir"`
	MaxMessageLength *int             `toml:"max_message_length"`
	DemoMode         *bool            `toml:"demo_mode"`
	LogLevel         *string          `toml:"log_level"`
	ReceiveLog       *tomlReceiveLog  `toml:"receive_log"`
	Stats            *tomlStats       `toml:"stats"`
	ChatLog          *tomlChatLog     `toml:"chat_log"`
	Send             *tomlSend        `toml:"send"`
	Forward          *tomlForward     `toml:"forward"`
	Auth             *tomlAuth        `toml:"auth"`
	RequestLog       *tomlRequestLog  `toml:"request_log"`
	Compression      *tomlCompression `toml:"compression"`
	Storage          *tomlStorage     `toml:"storage"`
}

type tomlReceiveLog struct {
	Enabled       *bool         `toml:"enabled"`
	Path          *string       `toml:"path"`
	RetentionDays *int          `toml:"retention_days"`
	ReplayWindow  *tomlDuration `toml:"replay_window"`
}

type tomlStats struct {
	Enabled       *bool   `toml:"enabled"`
	Path          *string `toml:"path"`
	RetentionDays *int    `toml:"retention_days"`
}

type tomlChatLog struct {
	Path             *string       `toml:"path"`
	HistoryWindow    *tomlDuration `toml:"history_window"`
	MaxHistoryWindow *tomlDuration `toml:"max_history_window"`
}

type tomlSend struct {
	DedupTTL *tomlDuration `toml:"dedup_ttl"`
}

type tomlForward struct {
	Targets *string `toml:"targets"`
}

type tomlAuth struct {
	Username   *string       `toml:"username"`
	Password   *string       `toml:"password"`
	SessionTTL *tomlDuration `toml:"session_ttl"`
	CookieName *string       `toml:"cookie_name"`
}

type tomlRequestLog struct {
	Enabled *bool `toml:"enabled"`
}

type tomlCompression struct {
	Enabled     *bool `toml:"enabled"`
	MinimumSize *int  `toml:"minimum_size"`
}

type tomlStorage struct {
	SQLitePath          *string       `toml:"sqlite_path"`
	PurgeInterval       *tomlDuration `toml:"purge_interval"`
	ReceiveLogRetention *tomlDuration `toml:"receive_log_retention"`
	PublicChatRetention *tomlDuration `toml:"public_chat_retention"`
	NodesRetention      *tomlDuration `toml:"nodes_retention"`
	TelemetryRetention  *tomlDuration `toml:"telemetry_retention"`
}

// tomlDuration encodes time.Duration as a human-readable string (e.g. "40s").
type tomlDuration struct{ time.Duration }

func (d *tomlDuration) UnmarshalText(b []byte) error {
	v, err := ParseDuration(string(b))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(b), err)
	}
	d.Duration = v
	return nil
}

func (d tomlDuration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func ParseDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasSuffix(trimmed, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(trimmed, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(trimmed)
}

// -------- Load --------

// loadTomlFile reads and parses the TOML config file at path.
// Returns nil with no error when the file does not exist.
// Returns an error when the file exists but contains invalid TOML.
func loadTomlFile(path string) (*tomlFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	var f tomlFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	return &f, nil
}

// -------- Env-override detection --------

// EnvOverrides records which config fields were explicitly provided via
// GOMESHCOM_* environment variables. Each key is the env var suffix
// (e.g. "MY_CALL"). A true value means the variable was present in the
// environment at startup.
type EnvOverrides map[string]bool

// knownEnvSuffixes lists every GOMESHCOM_* suffix that maps to a Config field.
var knownEnvSuffixes = []string{
	"HTTP_ADDR",
	"UDP_LISTEN_ADDR",
	"NODE_ADDR",
	"MY_CALL",
	"DATA_DIR",
	"SEND_DELAY",
	"MAX_MESSAGE_LENGTH",
	"LOG_LEVEL",
	"RECEIVE_LOG_ENABLED",
	"RECEIVE_LOG_PATH",
	"RECEIVE_LOG_RETENTION_DAYS",
	"RECEIVE_LOG_REPLAY_WINDOW",
	"STATS_ENABLED",
	"STATS_PATH",
	"STATS_RETENTION_DAYS",
	"CHAT_LOG_PATH",
	"CHAT_LOG_HISTORY_WINDOW",
	"CHAT_LOG_MAX_HISTORY_WINDOW",
	"DEMO_MODE",
	"SEND_DEDUP_TTL",
	"FORWARD_TARGETS",
	"AUTH_USERNAME",
	"AUTH_PASSWORD",
	"AUTH_SESSION_TTL",
	"AUTH_COOKIE_NAME",
	"REQUEST_LOG_ENABLED",
	"COMPRESSION_ENABLED",
	"COMPRESSION_MINIMUM_SIZE",
	"STORAGE_SQLITE_PATH",
	"STORAGE_PURGE_INTERVAL",
	"STORAGE_RECEIVE_LOG_RETENTION",
	"STORAGE_PUBLIC_CHAT_RETENTION",
	"STORAGE_NODES_RETENTION",
	"STORAGE_TELEMETRY_RETENTION",
}

// DetectEnvOverrides returns the set of config fields that are explicitly
// provided via GOMESHCOM_* environment variables in the current process.
func DetectEnvOverrides() EnvOverrides {
	m := make(EnvOverrides, len(knownEnvSuffixes))
	for _, suffix := range knownEnvSuffixes {
		_, m[suffix] = os.LookupEnv("GOMESHCOM_" + suffix)
	}
	return m
}

// -------- Merge --------

// mergeToml applies non-nil TOML file values to cfg, skipping fields that are
// explicitly set via environment variables (env[suffix] == true).
// DataDir is never overwritten from TOML to avoid a circular path dependency.
func mergeToml(cfg *Config, tf *tomlFile, env EnvOverrides) {
	if tf == nil {
		return
	}

	if tf.HTTPAddr != nil && !env["HTTP_ADDR"] {
		cfg.HTTPAddr = *tf.HTTPAddr
	}
	if tf.UDPListenAddr != nil && !env["UDP_LISTEN_ADDR"] {
		cfg.UDPListenAddr = *tf.UDPListenAddr
	}
	if tf.NodeAddr != nil && !env["NODE_ADDR"] {
		cfg.NodeAddr = *tf.NodeAddr
	}
	if tf.MyCall != nil && !env["MY_CALL"] {
		cfg.MyCall = *tf.MyCall
	}
	// DataDir deliberately not merged — TOML path is derived from DataDir,
	// so merging it would create a circular dependency.
	if tf.MaxMessageLength != nil && !env["MAX_MESSAGE_LENGTH"] {
		cfg.MaxMessageLength = *tf.MaxMessageLength
	}
	if tf.DemoMode != nil && !env["DEMO_MODE"] {
		cfg.DemoMode = *tf.DemoMode
	}
	if tf.LogLevel != nil && !env["LOG_LEVEL"] {
		cfg.LogLevel = *tf.LogLevel
	}

	if rl := tf.ReceiveLog; rl != nil {
		if rl.Enabled != nil && !env["RECEIVE_LOG_ENABLED"] {
			cfg.ReceiveLog.Enabled = *rl.Enabled
		}
		if rl.Path != nil && !env["RECEIVE_LOG_PATH"] {
			cfg.ReceiveLog.Path = *rl.Path
		}
		if rl.RetentionDays != nil && !env["RECEIVE_LOG_RETENTION_DAYS"] {
			cfg.ReceiveLog.RetentionDays = *rl.RetentionDays
		}
		if rl.ReplayWindow != nil && !env["RECEIVE_LOG_REPLAY_WINDOW"] {
			cfg.ReceiveLog.ReplayWindow = rl.ReplayWindow.Duration
		}
	}

	if s := tf.Stats; s != nil {
		if s.Enabled != nil && !env["STATS_ENABLED"] {
			cfg.Stats.Enabled = *s.Enabled
		}
		if s.Path != nil && !env["STATS_PATH"] {
			cfg.Stats.Path = *s.Path
		}
		if s.RetentionDays != nil && !env["STATS_RETENTION_DAYS"] {
			cfg.Stats.RetentionDays = *s.RetentionDays
		}
	}

	if cl := tf.ChatLog; cl != nil {
		if cl.Path != nil && !env["CHAT_LOG_PATH"] {
			cfg.ChatLog.Path = *cl.Path
		}
		if cl.HistoryWindow != nil && !env["CHAT_LOG_HISTORY_WINDOW"] {
			cfg.ChatLog.HistoryWindow = cl.HistoryWindow.Duration
		}
		if cl.MaxHistoryWindow != nil && !env["CHAT_LOG_MAX_HISTORY_WINDOW"] {
			cfg.ChatLog.MaxHistoryWindow = cl.MaxHistoryWindow.Duration
		}
	}

	if s := tf.Send; s != nil {
		if s.DedupTTL != nil && !env["SEND_DEDUP_TTL"] {
			cfg.Send.DedupTTL = s.DedupTTL.Duration
		}
	}

	if f := tf.Forward; f != nil {
		if f.Targets != nil && !env["FORWARD_TARGETS"] {
			cfg.Forward.Targets = *f.Targets
		}
	}

	if a := tf.Auth; a != nil {
		if a.Username != nil && !env["AUTH_USERNAME"] {
			cfg.Auth.Username = *a.Username
		}
		if a.Password != nil && !env["AUTH_PASSWORD"] {
			cfg.Auth.Password = *a.Password
		}
		if a.SessionTTL != nil && !env["AUTH_SESSION_TTL"] {
			cfg.Auth.SessionTTL = a.SessionTTL.Duration
		}
		if a.CookieName != nil && !env["AUTH_COOKIE_NAME"] {
			cfg.Auth.CookieName = *a.CookieName
		}
	}

	if rl := tf.RequestLog; rl != nil {
		if rl.Enabled != nil && !env["REQUEST_LOG_ENABLED"] {
			cfg.RequestLog.Enabled = *rl.Enabled
		}
	}

	if c := tf.Compression; c != nil {
		if c.Enabled != nil && !env["COMPRESSION_ENABLED"] {
			cfg.Compression.Enabled = *c.Enabled
		}
		if c.MinimumSize != nil && !env["COMPRESSION_MINIMUM_SIZE"] {
			cfg.Compression.MinimumSize = *c.MinimumSize
		}
	}

	if s := tf.Storage; s != nil {
		if s.SQLitePath != nil && !env["STORAGE_SQLITE_PATH"] {
			cfg.Storage.SQLitePath = *s.SQLitePath
		}
		if s.PurgeInterval != nil && !env["STORAGE_PURGE_INTERVAL"] {
			cfg.Storage.PurgeInterval = s.PurgeInterval.Duration
		}
		if s.ReceiveLogRetention != nil && !env["STORAGE_RECEIVE_LOG_RETENTION"] {
			cfg.Storage.ReceiveLogRetention = s.ReceiveLogRetention.Duration
		}
		if s.PublicChatRetention != nil && !env["STORAGE_PUBLIC_CHAT_RETENTION"] {
			cfg.Storage.PublicChatRetention = s.PublicChatRetention.Duration
		}
		if s.NodesRetention != nil && !env["STORAGE_NODES_RETENTION"] {
			cfg.Storage.NodesRetention = s.NodesRetention.Duration
		}
		if s.TelemetryRetention != nil && !env["STORAGE_TELEMETRY_RETENTION"] {
			cfg.Storage.TelemetryRetention = s.TelemetryRetention.Duration
		}
	}
}

// -------- Write --------

// WriteDefaultToml writes a fully-commented default TOML config to path.
// If the file already exists the call is a no-op.
// The parent directory is created if it does not exist.
// The write is atomic (tmp-file + rename).
//
// The generated file uses cfg as the value source (effective config at first
// startup), except auth.password which is always written as empty for safety.
func WriteDefaultToml(path string, cfg Config) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — do not overwrite
	}
	return writeTomlAtomically(path, buildTomlContent(cfg))
}

// WriteToml persists cfg to path as a fully-commented TOML file.
// The write is atomic. auth.password is always written as empty.
func WriteToml(path string, cfg Config) error {
	return writeTomlAtomically(path, buildTomlContent(cfg))
}

func writeTomlAtomically(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write temp config file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config file: %w", err)
	}

	return nil
}

// -------- Template generation --------

// buildTomlContent renders cfg as a deterministic, fully-commented TOML string.
// auth.password is always rendered as "" regardless of cfg.Auth.Password.
func buildTomlContent(cfg Config) string {
	cfg = normalize(cfg)
	var b strings.Builder

	w := func(comment, key, value string) {
		b.WriteString("# " + comment + "\n")
		b.WriteString(key + " = " + value + "\n")
	}

	b.WriteString("# gomeshcomd configuration file\n")
	b.WriteString("# Restart required unless stated otherwise.\n")
	b.WriteString("# Set GOMESHCOM_* env vars to override any field; env vars always win.\n")
	b.WriteString("\n")

	w("HTTP listen address", "http_addr", tomlStr(cfg.HTTPAddr))
	w("MeshCom UDP listen address", "udp_listen_addr", tomlStr(cfg.UDPListenAddr))
	w("MeshCom node UDP address; empty = auto-detect from first incoming packet", "node_addr", tomlStr(cfg.NodeAddr))
	w("Local callsign (live-apply — no restart needed)", "my_call", tomlStr(cfg.MyCall))
	w("Root data directory (ignored when set here — use GOMESHCOM_DATA_DIR env var)", "data_dir", tomlStr(cfg.DataDir))
	w("Max outgoing UTF-8 message length in bytes", "max_message_length", tomlInt(cfg.MaxMessageLength))
	w("Demo mode: disables TX and locks the config API (restart required)", "demo_mode", tomlBool(cfg.DemoMode))
	w("Log level: debug, info, warn, error (live-apply — no restart needed)", "log_level", tomlStr(cfg.LogLevel))

	b.WriteString("\n[receive_log]\n")
	w("Enable received UDP JSONL log", "enabled", tomlBool(cfg.ReceiveLog.Enabled))
	w("Directory for daily received UDP JSONL files", "path", tomlStr(cfg.ReceiveLog.Path))
	w("Days of daily log files to retain", "retention_days", tomlInt(cfg.ReceiveLog.RetentionDays))
	w("Window of recent packets replayed on SSE connect", "replay_window", tomlStr(cfg.ReceiveLog.ReplayWindow.String()))

	b.WriteString("\n[stats]\n")
	w("Enable hourly packet statistics collection", "enabled", tomlBool(cfg.Stats.Enabled))
	w("Path to stats JSON file", "path", tomlStr(cfg.Stats.Path))
	w("Days of hourly buckets to retain", "retention_days", tomlInt(cfg.Stats.RetentionDays))

	b.WriteString("\n[chat_log]\n")
	w("Chat JSONL directory", "path", tomlStr(cfg.ChatLog.Path))
	w("Default history window for GET /api/chat/{conversation} (live-apply)", "history_window", tomlStr(cfg.ChatLog.HistoryWindow.String()))
	w("Maximum history window allowed via ?hours= param (live-apply)", "max_history_window", tomlStr(cfg.ChatLog.MaxHistoryWindow.String()))

	b.WriteString("\n[send]\n")
	w("LRU window for duplicate outgoing message suppression; 0s disables (live-apply)", "dedup_ttl", tomlStr(cfg.Send.DedupTTL.String()))

	b.WriteString("\n[forward]\n")
	w("Comma-separated host:port list; received UDP datagrams mirrored to each target", "targets", tomlStr(cfg.Forward.Targets))

	b.WriteString("\n[auth]\n")
	w("HTTP basic auth username (both username and password required to enable auth)", "username", tomlStr(cfg.Auth.Username))
	// auth.password is always written as empty — never persist secrets in the config file.
	w("HTTP basic auth password (write-only — set via GOMESHCOM_AUTH_PASSWORD or PUT /api/config)", "password", tomlStr(""))
	w("Session cookie TTL", "session_ttl", tomlStr(cfg.Auth.SessionTTL.String()))
	w("Session cookie name", "cookie_name", tomlStr(cfg.Auth.CookieName))

	b.WriteString("\n[request_log]\n")
	w("Enable structured HTTP request logging (live-apply)", "enabled", tomlBool(cfg.RequestLog.Enabled))

	b.WriteString("\n[compression]\n")
	w("Enable HTTP gzip response compression (live-apply)", "enabled", tomlBool(cfg.Compression.Enabled))
	w("Minimum response body size in bytes before gzip is applied", "minimum_size", tomlInt(cfg.Compression.MinimumSize))

	b.WriteString("\n[storage]\n")
	w("Path to SQLite database file", "sqlite_path", tomlStr(cfg.Storage.SQLitePath))
	w("Interval between SQLite purge runs", "purge_interval", tomlStr(formatTomlDuration(cfg.Storage.PurgeInterval)))
	w("Retention window for SQLite receive_log rows", "receive_log_retention", tomlStr(formatTomlDuration(cfg.Storage.ReceiveLogRetention)))
	w("Retention window for SQLite public chat rows", "public_chat_retention", tomlStr(formatTomlDuration(cfg.Storage.PublicChatRetention)))
	w("Retention window for SQLite node rows based on lastseen", "nodes_retention", tomlStr(formatTomlDuration(cfg.Storage.NodesRetention)))
	w("Retention window for SQLite telemetry rows", "telemetry_retention", tomlStr(formatTomlDuration(cfg.Storage.TelemetryRetention)))
	b.WriteString("\n")

	return b.String()
}

func formatTomlDuration(d time.Duration) string {
	if d > 0 && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d > 0 && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return d.String()
}

func tomlStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func tomlInt(n int) string {
	return fmt.Sprintf("%d", n)
}

func tomlBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
