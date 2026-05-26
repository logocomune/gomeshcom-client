package channelshow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	path   string
	config Config
	dirty  bool
}

func New(path string) (*Store, error) {
	store := &Store{
		path:   path,
		config: DefaultConfig(),
	}
	if err := store.Load(); err != nil {
		return nil, err
	}
	return store, nil
}

func DefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "channel_show.json")
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
	_ = os.Remove(s.path + ".tmp")

	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open channel show: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return fmt.Errorf("decode channel show: %w", err)
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

	if err := writeFileAtomically(s.path, snapshot); err != nil {
		s.mu.Lock()
		s.dirty = true
		s.mu.Unlock()
		return err
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

func writeFileAtomically(path string, config Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create channel show dir: %w", err)
	}

	tmpPath := path + ".tmp"
	temp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp channel show file: %w", err)
	}

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if encErr := encoder.Encode(config); encErr != nil {
		temp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode channel show: %w", encErr)
	}

	if err := temp.Sync(); err != nil {
		temp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp channel show file: %w", err)
	}

	if err := temp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp channel show file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace channel show file: %w", err)
	}

	return nil
}
