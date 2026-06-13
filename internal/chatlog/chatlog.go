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

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)
var validID = regexp.MustCompile(`^(P_broadcast|P_\d+|DM_[A-Z0-9_-]+)$`)

// ErrInvalidID is returned by ReadSince when the conversation ID fails validation.
var ErrInvalidID = errors.New("invalid conversation id")

// myCallSource provides the live local callsign. *station.Identity satisfies
// this interface; a static string wrapper may be used in tests.
type myCallSource interface {
	Current() string
}

type Logger struct {
	mu       sync.Mutex
	baseDir  string
	identity myCallSource
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

func New(baseDir string, identity myCallSource) *Logger {
	return &Logger{baseDir: baseDir, identity: identity}
}

// myCall returns the current local callsign, or "" if no identity is set.
func (l *Logger) myCall() string {
	if l.identity == nil {
		return ""
	}
	return l.identity.Current()
}

func (l *Logger) Append(msg meshcom.TextMessage, receivedAt time.Time) error {
	name := filenameForMsg(msg.Source, msg.Destination, l.myCall())
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

// ConversationID returns the .jsonl file identifier (without suffix) for a
// message from src to dst, normalised relative to myCall. For DMs this is
// the basecall-namespaced file id, e.g. DM_IU5PMP_IK5FCK-10.
// Returns "" for DMs that do not involve myCall.
func ConversationID(src, dst, myCall string) string {
	return strings.TrimSuffix(filenameForMsg(src, dst, myCall), ".jsonl")
}

// filenameForMsg returns the JSONL filename for a message. DM files are keyed
// on the operator base callsign (BaseCall(myCall)) so that device switches
// (e.g. IU5PMP-1 → IU5PMP-2) share the same conversation history. Returns ""
// when the message should be silently dropped (DM not involving myCall).
func filenameForMsg(src, dst, myCall string) string {
	if !IsDM(dst) {
		return filename(dst)
	}
	// DM path — no myCall configured: no filter, use dst as before.
	if myCall == "" {
		return filename(dst)
	}
	origin := strings.ToUpper(strings.SplitN(src, ",", 2)[0])
	dstUpper := strings.ToUpper(dst)
	myBase := BaseCall(myCall)
	if BaseCall(origin) != myBase && BaseCall(dstUpper) != myBase {
		return "" // not our conversation
	}
	interlocutor := dstUpper
	if BaseCall(dstUpper) == myBase {
		interlocutor = origin
	}
	return "DM_" + sanitize(BaseCall(myCall)) + "_" + sanitize(interlocutor) + ".jsonl"
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

// IsDM reports whether dst is a direct-message destination (non-empty, not
// broadcast "*", and not a numeric channel identifier).
func IsDM(dst string) bool {
	return dst != "" && dst != "*" && !isNumeric(dst)
}

func sanitize(s string) string {
	return strings.ToUpper(unsafeChars.ReplaceAllString(s, "_"))
}

// Sanitize uppercases s and replaces characters outside [A-Za-z0-9_-] with "_".
// Exported for use by packages that must build conversation IDs externally.
func Sanitize(s string) string { return sanitize(s) }

// BaseCall strips a numeric SSID suffix from a callsign.
// IU5PMP-1 → IU5PMP, IU5PMP-10 → IU5PMP, IU5PMP → IU5PMP.
// A non-numeric suffix (e.g. "-X") is left intact.
func BaseCall(callsign string) string {
	if i := strings.LastIndex(callsign, "-"); i >= 0 {
		if isNumeric(callsign[i+1:]) {
			return callsign[:i]
		}
	}
	return callsign
}

// StatusKey returns the msg_idx.json key for a message, using the full mycall
// SSID so that read-state is tracked per device. Returns "" for DMs that do not
// involve myCall. Non-DM conversations return the same P_* id as ConversationID.
func StatusKey(src, dst, myCall string) string {
	if !IsDM(dst) {
		return strings.TrimSuffix(filename(dst), ".jsonl")
	}
	if myCall == "" {
		return ""
	}
	myCall = strings.ToUpper(myCall)
	origin := strings.ToUpper(strings.SplitN(src, ",", 2)[0])
	dstUpper := strings.ToUpper(dst)
	myBase := BaseCall(myCall)
	if BaseCall(origin) != myBase && BaseCall(dstUpper) != myBase {
		return "" // not our conversation
	}
	interlocutor := dstUpper
	if BaseCall(dstUpper) == myBase {
		interlocutor = origin
	}
	return "DM_" + sanitize(myCall) + "_" + sanitize(interlocutor)
}

// FileIDForAPIID derives the .jsonl file conversation ID from an API conversation id.
// For DM ids of the form DM_<caller>_<peer>, it returns DM_<BaseCall(caller)>_<peer>.
// For P_* ids and legacy DM_<peer> ids (no underscore in rest), returns id unchanged.
func FileIDForAPIID(id string) string {
	if !strings.HasPrefix(id, "DM_") {
		return id
	}
	rest := id[3:]
	idx := strings.Index(rest, "_")
	if idx < 0 {
		return id // legacy DM_<peer>: no separator
	}
	caller := rest[:idx]
	peer := rest[idx+1:]
	return "DM_" + sanitize(BaseCall(caller)) + "_" + peer
}

// DMPeer extracts the peer callsign segment from a DM conversation id.
// For DM_<a>_<peer> it returns <peer>; for legacy DM_<peer> it returns <peer>.
// Returns "" for non-DM ids.
func DMPeer(id string) string {
	if !strings.HasPrefix(id, "DM_") {
		return ""
	}
	rest := id[3:]
	if idx := strings.Index(rest, "_"); idx >= 0 {
		return rest[idx+1:]
	}
	return rest // legacy DM_<peer>
}

// RecordMatchesSSID reports whether a chat record's my-side callsign equals ssid.
// It checks both Src and Dst, stripping routing-path suffixes (comma-separated).
func RecordMatchesSSID(src, dst, ssid string) bool {
	ssid = strings.ToUpper(ssid)
	return strings.ToUpper(strings.SplitN(src, ",", 2)[0]) == ssid ||
		strings.ToUpper(dst) == ssid
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
			// New format: DM_<basecall>_<peer>. Extract peer as the label.
			// Legacy format DM_<peer> (no underscore): peer == rest.
			rest := strings.TrimPrefix(id, "DM_")
			if idx := strings.Index(rest, "_"); idx >= 0 {
				rest = rest[idx+1:]
			}
			conv.Label = rest
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

// FileContainsSSID returns true if the JSONL file for fileID has at least one
// record where Src or Dst matches ssid (full callsign with SSID). Used by the
// list endpoint to exclude basecall-shared files that have no record for the
// active SSID under mycall scope.
func (l *Logger) FileContainsSSID(fileID, ssid string) (bool, error) {
	l.mu.Lock()
	path := filepath.Join(l.baseDir, fileID+".jsonl")
	l.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open chat log %s: %w", fileID, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec Record
		if json.Unmarshal(scanner.Bytes(), &rec) != nil {
			continue
		}
		if RecordMatchesSSID(rec.Src, rec.Dst, ssid) {
			return true, nil
		}
	}
	return false, scanner.Err()
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
