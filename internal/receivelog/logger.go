package receivelog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const fileDateLayout = "20060102"

type Config struct {
	Enabled       bool
	Path          string
	RetentionDays int
}

type Logger struct {
	cfg Config
	mu  sync.Mutex
}

type Record struct {
	ReceivedAt time.Time `json:"received_at"`
	RemoteAddr string    `json:"remote_addr"`
	Bytes      int       `json:"bytes"`
	Raw        string    `json:"raw"`
	PacketType string    `json:"packet_type,omitempty"`
	ParseError string    `json:"parse_error,omitempty"`
}

func New(cfg Config) *Logger {
	return &Logger{cfg: cfg}
}

func (l *Logger) Append(record Record) error {
	if !l.cfg.Enabled {
		return nil
	}

	if err := l.validate(); err != nil {
		return err
	}

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal receive log record: %w", err)
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	logPath := l.pathForRecord(record, now)

	if err := os.MkdirAll(l.logDir(), 0o755); err != nil {
		return fmt.Errorf("create receive log dir: %w", err)
	}

	if err := l.pruneExpired(now); err != nil {
		return err
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open receive log: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("write receive log: %w", err)
	}

	return nil
}

func (l *Logger) ReadSince(cutoff time.Time) ([]Record, error) {
	if !l.cfg.Enabled {
		return nil, nil
	}

	if err := l.validate(); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var records []Record
	for _, path := range l.pathsSince(cutoff, time.Now().UTC()) {
		fileRecords, err := readRecordsSince(path, cutoff)
		if err != nil {
			return nil, err
		}
		records = append(records, fileRecords...)
	}

	return records, nil
}

func (l *Logger) validate() error {
	if l.cfg.Path == "" {
		return errors.New("receive log path is required")
	}

	if l.cfg.RetentionDays < 0 {
		return errors.New("receive log retention days must not be negative")
	}

	return nil
}

func (l *Logger) pathForRecord(record Record, fallback time.Time) string {
	receivedAt := record.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = fallback
	}
	return l.pathForDate(receivedAt)
}

func (l *Logger) pathForDate(receivedAt time.Time) string {
	name := "received." + receivedAt.UTC().Format(fileDateLayout) + ".jsonl"
	return filepath.Join(l.logDir(), name)
}

func (l *Logger) logDir() string {
	if filepath.Ext(l.cfg.Path) == ".jsonl" {
		return filepath.Dir(l.cfg.Path)
	}
	return l.cfg.Path
}

func (l *Logger) pathsSince(cutoff time.Time, now time.Time) []string {
	start := dayStart(cutoff)
	end := dayStart(now)
	if start.After(end) {
		return nil
	}

	var paths []string
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		paths = append(paths, l.pathForDate(day))
	}
	return paths
}

func dayStart(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func readRecordsSince(path string, cutoff time.Time) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open receive log: %w", err)
	}
	defer file.Close()

	records := make([]Record, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode receive log record: %w", err)
		}
		if !record.ReceivedAt.Before(cutoff) {
			records = append(records, record)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan receive log: %w", err)
	}

	return records, nil
}

func (l *Logger) pruneExpired(now time.Time) error {
	if l.cfg.RetentionDays == 0 {
		return nil
	}

	cutoff := dayStart(now).AddDate(0, 0, -l.cfg.RetentionDays+1)
	paths, err := filepath.Glob(filepath.Join(l.logDir(), "received.*.jsonl"))
	if err != nil {
		return fmt.Errorf("list receive logs: %w", err)
	}
	sort.Strings(paths)

	for _, path := range paths {
		if err := pruneFile(path, cutoff); err != nil {
			return err
		}
	}

	return nil
}

func pruneFile(path string, cutoff time.Time) error {
	records, err := readRecordsSince(path, cutoff)
	if err != nil {
		return err
	}
	if records == nil {
		return nil
	}
	if len(records) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove expired receive log: %w", err)
		}
		return nil
	}

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create pruned receive log: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	encoder := json.NewEncoder(temp)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			temp.Close()
			return fmt.Errorf("encode pruned receive log: %w", err)
		}
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close pruned receive log: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace pruned receive log: %w", err)
	}

	return nil
}
