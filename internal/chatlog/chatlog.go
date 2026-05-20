package chatlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-udp/internal/meshcom"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)
var validID = regexp.MustCompile(`^(P_broadcast|P_\d+|DM_[A-Z0-9_-]+)$`)

// ErrInvalidID is returned by ReadSince when the conversation ID fails validation.
var ErrInvalidID = errors.New("invalid conversation id")

type Logger struct {
	mu      sync.Mutex
	baseDir string
	myCall  string
}

type Record struct {
	ReceivedAt     time.Time `json:"received_at"`
	Src            string    `json:"src,omitempty"`
	SrcType        string    `json:"src_type,omitempty"`
	Dst            string    `json:"dst,omitempty"`
	MsgID          string    `json:"msg_id,omitempty"`
	Msg            string    `json:"msg"`
	RSSI           int       `json:"rssi,omitempty"`
	SNR            int       `json:"snr,omitempty"`
	Direction      string    `json:"direction,omitempty"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
}

func New(baseDir string, myCall string) *Logger {
	return &Logger{baseDir: baseDir, myCall: strings.ToUpper(myCall)}
}

func (l *Logger) Append(msg meshcom.TextMessage, receivedAt time.Time) error {
	name := filenameForMsg(msg.Source, msg.Destination, l.myCall)
	if name == "" {
		return nil
	}

	rec := Record{
		ReceivedAt: receivedAt.UTC(),
		Src:        msg.Source,
		SrcType:    msg.SourceType,
		Dst:        msg.Destination,
		MsgID:      msg.MessageID,
		Msg:        msg.Message,
	}
	if msg.RSSI != nil {
		rec.RSSI = *msg.RSSI
	}
	if msg.SNR != nil {
		rec.SNR = *msg.SNR
	}

	return l.appendRecord(name, rec)
}

func (l *Logger) AppendFailed(source, destination, message string, receivedAt time.Time) (Record, error) {
	rec := Record{
		ReceivedAt:     receivedAt.UTC(),
		Src:            source,
		Dst:            destination,
		Msg:            message,
		Direction:      "outbound",
		DeliveryStatus: "failed",
	}

	name := filenameForMsg(source, destination, source)
	if name == "" {
		return rec, nil
	}

	return rec, l.appendRecord(name, rec)
}

func (l *Logger) appendRecord(name string, rec Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal chat log record: %w", err)
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(l.baseDir, 0o755); err != nil {
		return fmt.Errorf("create chat log dir: %w", err)
	}

	path := filepath.Join(l.baseDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open chat log: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("write chat log: %w", err)
	}

	return nil
}

// filenameForMsg returns the JSONL filename for a message, normalising DM
// conversations on the interlocutor's callsign so both directions of a DM
// land in the same file. Returns "" when the message should be silently
// dropped (DM not involving myCall).
func filenameForMsg(src, dst, myCall string) string {
	if !isDM(dst) {
		return filename(dst)
	}
	// DM path — no myCall configured: no filter, use dst as before.
	if myCall == "" {
		return filename(dst)
	}
	origin := strings.ToUpper(strings.SplitN(src, ",", 2)[0])
	dstUpper := strings.ToUpper(dst)
	if origin != myCall && dstUpper != myCall {
		return "" // not our conversation
	}
	interlocutor := dstUpper
	if dstUpper == myCall {
		interlocutor = origin
	}
	return "DM_" + sanitize(interlocutor) + ".jsonl"
}

func filename(dst string) string {
	if dst == "" || dst == "*" {
		return "P_broadcast.jsonl"
	}
	if isNumeric(dst) {
		return "P_" + dst + ".jsonl"
	}
	return "DM_" + sanitize(dst) + ".jsonl"
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isDM(dst string) bool {
	return dst != "" && dst != "*" && !isNumeric(dst)
}

func sanitize(s string) string {
	return strings.ToUpper(unsafeChars.ReplaceAllString(s, "_"))
}

// Conversation describes a discovered chat log file.
type Conversation struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Label    string    `json:"label"`
	LastSeen time.Time `json:"last_seen"`
	Size     int64     `json:"size"`
}

// List enumerates conversation JSONL files in baseDir.
// Returns an empty slice (not an error) when the directory does not exist.
func (l *Logger) List() ([]Conversation, error) {
	l.mu.Lock()
	dir := l.baseDir
	l.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Conversation{}, nil
		}
		return nil, fmt.Errorf("read chat dir: %w", err)
	}

	var convs []Conversation
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		if !validID.MatchString(id) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		conv := Conversation{
			ID:       id,
			LastSeen: info.ModTime().UTC(),
			Size:     info.Size(),
		}
		switch {
		case id == "P_broadcast":
			conv.Kind = "broadcast"
			conv.Label = "Broadcast"
		case strings.HasPrefix(id, "P_"):
			conv.Kind = "channel"
			conv.Label = strings.TrimPrefix(id, "P_")
		default:
			conv.Kind = "dm"
			conv.Label = strings.TrimPrefix(id, "DM_")
		}
		convs = append(convs, conv)
	}

	sort.Slice(convs, func(i, j int) bool {
		return convs[i].LastSeen.After(convs[j].LastSeen)
	})

	return convs, nil
}

// ValidConversationID reports whether id matches the allowed pattern.
func ValidConversationID(id string) bool {
	return validID.MatchString(id)
}

// Remove deletes the JSONL file for the given conversation ID.
// Returns nil if the file does not exist (idempotent).
func (l *Logger) Remove(id string) error {
	if !validID.MatchString(id) {
		return ErrInvalidID
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	path := filepath.Join(l.baseDir, id+".jsonl")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove chat log %s: %w", id, err)
	}
	return nil
}

// ReadSince returns records from the conversation JSONL file with ReceivedAt >= since,
// sorted ascending by ReceivedAt. Malformed lines are skipped with a warning.
func (l *Logger) ReadSince(id string, since time.Time) ([]Record, error) {
	if !validID.MatchString(id) {
		return nil, ErrInvalidID
	}

	l.mu.Lock()
	path := filepath.Join(l.baseDir, id+".jsonl")
	l.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, fmt.Errorf("open chat log %s: %w", id, err)
	}
	defer file.Close()

	seen := make(map[string]struct{})
	records := make([]Record, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			slog.Warn("chat log malformed line", "id", id, "error", err)
			continue
		}
		if rec.MsgID != "" {
			if _, dup := seen[rec.MsgID]; dup {
				continue
			}
			seen[rec.MsgID] = struct{}{}
		}
		if !rec.ReceivedAt.Before(since) {
			records = append(records, rec)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan chat log %s: %w", id, err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].ReceivedAt.Before(records[j].ReceivedAt)
	})

	return records, nil
}
