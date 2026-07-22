package channelshow

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ModeAll       = "all"
	ModeAllowlist = "allowlist"

	saveInterval = time.Minute
)

// Config controls which public channels the UI should show.
type Config struct {
	Mode     string   `json:"mode"`
	Channels []string `json:"channels"`
}

type Store struct {
	mu     sync.Mutex
	db     *sql.DB
	config Config
	dirty  bool
}

func NewSQLite(db *sql.DB) (*Store, error) {
	store := &Store{
		db:     db,
		config: DefaultConfig(),
	}
	if err := store.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "configs", "channel_show.json")
}

func DefaultConfig() Config {
	return Config{Mode: ModeAll, Channels: []string{}}
}

func Normalize(config Config) (Config, error) {
	mode := strings.ToLower(strings.TrimSpace(config.Mode))
	if mode == "" {
		if len(config.Channels) > 0 {
			mode = ModeAllowlist
		} else {
			mode = ModeAll
		}
	}

	switch mode {
	case ModeAll:
		return DefaultConfig(), nil
	case ModeAllowlist:
		channels, err := NormalizeChannels(config.Channels)
		if err != nil {
			return Config{}, err
		}
		return Config{Mode: ModeAllowlist, Channels: channels}, nil
	default:
		return Config{}, fmt.Errorf("invalid mode %q", config.Mode)
	}
}

func NormalizeChannels(channels []string) ([]string, error) {
	normalized := make([]string, 0, len(channels))
	seen := make(map[string]bool, len(channels))
	for _, raw := range channels {
		channel := strings.TrimSpace(raw)
		if !ValidChannel(channel) {
			return nil, fmt.Errorf("invalid channel %q", raw)
		}
		if seen[channel] {
			continue
		}
		seen[channel] = true
		normalized = append(normalized, channel)
	}
	if normalized == nil {
		normalized = []string{}
	}
	return normalized, nil
}

func ValidChannel(channel string) bool {
	if channel == "*" {
		return true
	}
	if channel == "" {
		return false
	}
	for _, r := range channel {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Store) Load() error {
	return s.loadSQLite()
}

func (s *Store) Snapshot() Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	return cloneConfig(s.config)
}

func (s *Store) Update(config Config) (Config, error) {
	normalized, err := Normalize(config)
	if err != nil {
		return Config{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if configsEqual(s.config, normalized) {
		return cloneConfig(s.config), nil
	}

	s.config = cloneConfig(normalized)
	s.dirty = true
	return cloneConfig(s.config), nil
}

func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("channel show flush failed", "error", err)
			}
			return
		case <-ticker.C:
			if err := s.SaveIfDirty(); err != nil {
				slog.Error("channel show save failed", "error", err)
			}
		}
	}
}

func (s *Store) SaveIfDirty() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	snapshot := cloneConfig(s.config)
	s.dirty = false
	s.mu.Unlock()

	if err := writeSQLite(s.db, snapshot); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
	}

	return nil
}

func (s *Store) loadSQLite() error {
	var mode string
	err := s.db.QueryRow(`SELECT mode FROM channel_show WHERE id = 1`).Scan(&mode)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("query channel show: %w", err)
	}
	config := DefaultConfig()
	if err != sql.ErrNoRows {
		config.Mode = mode
	}

	rows, err := s.db.Query(`SELECT channel FROM channel_show_channels ORDER BY channel`)
	if err != nil {
		return fmt.Errorf("query channel show channels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err != nil {
			return fmt.Errorf("scan channel show channel: %w", err)
		}
		config.Channels = append(config.Channels, channel)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate channel show channels: %w", err)
	}

	normalized, err := Normalize(config)
	if err != nil {
		return fmt.Errorf("validate channel show: %w", err)
	}
	s.mu.Lock()
	s.config = normalized
	s.dirty = false
	s.mu.Unlock()
	return nil
}

func writeSQLite(db *sql.DB, config Config) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin channel show save: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO channel_show(id, mode) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET mode = excluded.mode`, config.Mode); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save channel show mode: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM channel_show_channels`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear channel show channels: %w", err)
	}
	for _, channel := range config.Channels {
		if _, err := tx.Exec(`INSERT INTO channel_show_channels(channel, last_message_at) VALUES (?, NULL)`, channel); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("save channel show channel %s: %w", channel, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit channel show save: %w", err)
	}
	return nil
}

func cloneConfig(config Config) Config {
	channels := make([]string, len(config.Channels))
	copy(channels, config.Channels)
	return Config{Mode: config.Mode, Channels: channels}
}

func configsEqual(left, right Config) bool {
	if left.Mode != right.Mode || len(left.Channels) != len(right.Channels) {
		return false
	}
	for i := range left.Channels {
		if left.Channels[i] != right.Channels[i] {
			return false
		}
	}
	return true
}
