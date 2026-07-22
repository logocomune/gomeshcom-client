package chatlog

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/meshcom"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)
var validID = regexp.MustCompile(`^(P_broadcast|P_\d+|DM_[A-Z0-9_-]+)$`)
var messageSequencePattern = regexp.MustCompile(`\{(\d+)\s*$`)
var ackSequencePattern = regexp.MustCompile(`(?i)(?:^|[:\s])(?:ack)?(\d+)\b`)
var rejectSequencePattern = regexp.MustCompile(`(?i)(?:^|[:\s])rej(\d+)\b`)

const recordDedupeWindow = 5 * time.Minute

// ErrInvalidID is returned by ReadSince when the conversation ID fails validation.
var ErrInvalidID = errors.New("invalid conversation id")

// myCallSource provides the live local callsign. *station.Identity satisfies
// this interface; a static string wrapper may be used in tests.
type myCallSource interface {
	Current() string
}

type Logger struct {
	mu       sync.Mutex
	db       *sql.DB
	identity myCallSource
}

type Record struct {
	ReceivedAt     time.Time `json:"received_at"`
	Src            string    `json:"src,omitempty"`
	SrcType        string    `json:"src_type,omitempty"`
	Via            []string  `json:"via,omitempty"`
	Dst            string    `json:"dst,omitempty"`
	MsgID          string    `json:"msg_id,omitempty"`
	SequenceID     string    `json:"sequence_id,omitempty"`
	Msg            string    `json:"msg"`
	RSSI           int       `json:"rssi,omitempty"`
	SNR            int       `json:"snr,omitempty"`
	Direction      string    `json:"direction,omitempty"`
	DeliveryStatus string    `json:"delivery_status,omitempty"`
	AckStatus      string    `json:"ack_status,omitempty"`
	AckReceivedAt  time.Time `json:"ack_received_at,omitempty"`
	AckSrc         string    `json:"ack_src,omitempty"`
	AckSrcType     string    `json:"ack_src_type,omitempty"`
	AckRSSI        int       `json:"ack_rssi,omitempty"`
	AckSNR         int       `json:"ack_snr,omitempty"`
	AckVia         []string  `json:"ack_via,omitempty"`
}

func NewSQLite(db *sql.DB, identity myCallSource) *Logger {
	return &Logger{db: db, identity: identity}
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

	origin, relays := sourceOriginAndRelays(msg.Source)
	rec := Record{
		ReceivedAt: receivedAt.UTC(),
		Src:        origin,
		SrcType:    msg.SourceType,
		Via:        relays,
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
	rec.SequenceID = messageSequenceID(rec.Msg)

	return l.appendRecord(name, rec)
}

func (l *Logger) AppendFailed(source, destination, message string, receivedAt time.Time) (Record, error) {
	origin, relays := sourceOriginAndRelays(source)
	rec := Record{
		ReceivedAt:     receivedAt.UTC(),
		Src:            origin,
		Via:            relays,
		Dst:            destination,
		Msg:            message,
		SequenceID:     messageSequenceID(message),
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
	return l.appendSQLite(strings.TrimSuffix(name, ".jsonl"), rec)
}

func (l *Logger) appendSQLite(conversationID string, rec Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if strings.HasPrefix(conversationID, "DM_") {
		_, err := l.db.Exec(`
			INSERT INTO chats_dm(conversation_id, msg_id, sequence_id, received_at, src, src_type, via, dst, msg, rssi, snr, direction, delivery_status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, conversationID, nullableString(rec.MsgID), nullableString(rec.SequenceID), rec.ReceivedAt.UTC().Format(time.RFC3339Nano), nullableString(rec.Src), nullableString(rec.SrcType), nullableStringSlice(rec.Via), rec.Dst, rec.Msg, nullableInt(rec.RSSI), nullableInt(rec.SNR), nullableString(rec.Direction), nullableString(rec.DeliveryStatus))
		if err != nil {
			return fmt.Errorf("insert dm chat record: %w", err)
		}
		if err := l.updateRecentOutboundAck(conversationID, rec); err != nil {
			return err
		}
		return nil
	}

	kind, channel := publicKind(conversationID)
	_, err := l.db.Exec(`
		INSERT INTO chats_public(conversation_id, kind, channel, msg_id, received_at, src, src_type, via, dst, msg, rssi, snr)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, conversationID, kind, channel, nullableString(rec.MsgID), rec.ReceivedAt.UTC().Format(time.RFC3339Nano), nullableString(rec.Src), nullableString(rec.SrcType), nullableStringSlice(rec.Via), rec.Dst, rec.Msg, nullableInt(rec.RSSI), nullableInt(rec.SNR))
	if err != nil {
		return fmt.Errorf("insert public chat record: %w", err)
	}
	return nil
}

func publicKind(conversationID string) (string, any) {
	if conversationID == "P_broadcast" {
		return "broadcast", nil
	}
	return "channel", strings.TrimPrefix(conversationID, "P_")
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableStringSlice(values []string) any {
	if len(values) == 0 {
		return nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return string(encoded)
}

func messageSequenceID(message string) string {
	match := messageSequencePattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func ackStatusAndSequenceID(message string) (string, string) {
	if match := rejectSequencePattern.FindStringSubmatch(message); len(match) == 2 {
		return "reject", match[1]
	}
	if match := ackSequencePattern.FindStringSubmatch(message); len(match) == 2 {
		return "ack", match[1]
	}
	return "", ""
}

func sourceOriginAndRelays(source string) (string, []string) {
	parts := strings.Split(source, ",")
	origin := strings.ToUpper(strings.TrimSpace(parts[0]))
	relays := make([]string, 0, len(parts)-1)
	for _, part := range parts[1:] {
		relay := strings.ToUpper(strings.TrimSpace(part))
		if relay != "" {
			relays = append(relays, relay)
		}
	}
	return origin, relays
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

// List enumerates stored conversations.
// Returns an empty slice (not an error) when the directory does not exist.
func (l *Logger) List() ([]Conversation, error) {
	return l.listSQLite()
}

func (l *Logger) listSQLite() ([]Conversation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rows, err := l.db.Query(`
		SELECT conversation_id, MAX(received_at), COUNT(*) FROM chats_public GROUP BY conversation_id
		UNION ALL
		SELECT conversation_id, MAX(received_at), COUNT(*) FROM chats_dm GROUP BY conversation_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query chat conversations: %w", err)
	}
	defer rows.Close()

	convs := []Conversation{}
	for rows.Next() {
		var id string
		var lastSeen string
		var size int64
		if err := rows.Scan(&id, &lastSeen, &size); err != nil {
			return nil, fmt.Errorf("scan chat conversation: %w", err)
		}
		if !validID.MatchString(id) {
			continue
		}
		parsedLastSeen, err := time.Parse(time.RFC3339Nano, lastSeen)
		if err != nil {
			return nil, fmt.Errorf("parse chat conversation timestamp %s: %w", id, err)
		}
		conv := Conversation{ID: id, LastSeen: parsedLastSeen, Size: size}
		applyConversationLabel(&conv)
		convs = append(convs, conv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat conversations: %w", err)
	}
	sort.Slice(convs, func(i, j int) bool { return convs[i].LastSeen.After(convs[j].LastSeen) })
	return convs, nil
}

func applyConversationLabel(conv *Conversation) {
	switch {
	case conv.ID == "P_broadcast":
		conv.Kind = "broadcast"
		conv.Label = "Broadcast"
	case strings.HasPrefix(conv.ID, "P_"):
		conv.Kind = "channel"
		conv.Label = strings.TrimPrefix(conv.ID, "P_")
	default:
		conv.Kind = "dm"
		rest := strings.TrimPrefix(conv.ID, "DM_")
		if idx := strings.Index(rest, "_"); idx >= 0 {
			rest = rest[idx+1:]
		}
		conv.Label = rest
	}
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
	return l.removeSQLite(id)
}

// FileContainsSSID returns true if the JSONL file for fileID has at least one
// record where Src or Dst matches ssid (full callsign with SSID). Used by the
// list endpoint to exclude basecall-shared files that have no record for the
// active SSID under mycall scope.
func (l *Logger) FileContainsSSID(fileID, ssid string) (bool, error) {
	return l.fileContainsSSIDSQLite(fileID, ssid)
}

func (l *Logger) removeSQLite(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if strings.HasPrefix(id, "DM_") {
		_, err := l.db.Exec(`DELETE FROM chats_dm WHERE conversation_id = ?`, id)
		if err != nil {
			return fmt.Errorf("remove dm chat log %s: %w", id, err)
		}
		return nil
	}
	_, err := l.db.Exec(`DELETE FROM chats_public WHERE conversation_id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove public chat log %s: %w", id, err)
	}
	return nil
}

func (l *Logger) fileContainsSSIDSQLite(fileID, ssid string) (bool, error) {
	if !validID.MatchString(fileID) {
		return false, ErrInvalidID
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	query := `SELECT src, dst FROM chats_public WHERE conversation_id = ?`
	if strings.HasPrefix(fileID, "DM_") {
		query = `SELECT src, dst FROM chats_dm WHERE conversation_id = ?`
	}
	rows, err := l.db.Query(query, fileID)
	if err != nil {
		return false, fmt.Errorf("query chat log %s: %w", fileID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var src, dst sql.NullString
		if err := rows.Scan(&src, &dst); err != nil {
			return false, fmt.Errorf("scan chat log %s: %w", fileID, err)
		}
		if RecordMatchesSSID(src.String, dst.String, ssid) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func isDuplicateRecord(seen []Record, candidate Record) bool {
	if candidate.MsgID == "" {
		return false
	}
	for _, existing := range seen {
		if existing.MsgID == candidate.MsgID && originCall(existing.Src) == originCall(candidate.Src) && receivedWithinDedupeWindow(existing.ReceivedAt, candidate.ReceivedAt) {
			return true
		}
	}
	return false
}

func originCall(source string) string {
	return strings.ToUpper(strings.TrimSpace(strings.SplitN(source, ",", 2)[0]))
}

func receivedWithinDedupeWindow(left, right time.Time) bool {
	delta := left.Sub(right)
	if delta < 0 {
		delta = -delta
	}
	return delta <= recordDedupeWindow
}

func (l *Logger) updateRecentOutboundAck(conversationID string, rec Record) error {
	status, sequenceID := ackStatusAndSequenceID(rec.Msg)
	if status == "" || sequenceID == "" {
		return nil
	}
	origin := strings.ToUpper(strings.TrimSpace(rec.Src))
	relays := rec.Via
	if origin == "" {
		return nil
	}

	ackAt := rec.ReceivedAt.UTC()
	ackAtText := ackAt.Format(time.RFC3339Nano)
	cutoffText := ackAt.Add(-recordDedupeWindow).Format(time.RFC3339Nano)
	_, err := l.db.Exec(`
		UPDATE chats_dm
		SET ack_status = ?,
			ack_received_at = ?,
			ack_src = ?,
			ack_src_type = ?,
			ack_rssi = ?,
			ack_snr = ?,
			ack_via = ?
		WHERE id = (
			SELECT id
			FROM chats_dm
			WHERE conversation_id = ?
				AND direction = 'outbound'
				AND sequence_id = ?
				AND ack_status IS NULL
				AND UPPER(dst) = UPPER(?)
				AND received_at >= ?
				AND received_at <= ?
			ORDER BY received_at DESC, id DESC
			LIMIT 1
		)
	`, status, ackAtText, nullableString(rec.Src), nullableString(rec.SrcType), nullableInt(rec.RSSI), nullableInt(rec.SNR), nullableStringSlice(relays), conversationID, sequenceID, origin, cutoffText, ackAtText)
	if err != nil {
		return fmt.Errorf("update dm ack status: %w", err)
	}
	return nil
}

func (l *Logger) readSinceSQLite(id string, since time.Time) ([]Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	query := `
		SELECT received_at, src, src_type, via, dst, msg_id, NULL AS sequence_id, msg, rssi, snr,
			NULL AS direction, NULL AS delivery_status, NULL AS ack_status, NULL AS ack_received_at,
			NULL AS ack_src, NULL AS ack_src_type, NULL AS ack_rssi, NULL AS ack_snr, NULL AS ack_via
		FROM chats_public
		WHERE conversation_id = ?
		ORDER BY received_at, id
	`
	if strings.HasPrefix(id, "DM_") {
		query = `
			SELECT received_at, src, src_type, via, dst, msg_id, sequence_id, msg, rssi, snr,
				direction, delivery_status, ack_status, ack_received_at,
				ack_src, ack_src_type, ack_rssi, ack_snr, ack_via
			FROM chats_dm
			WHERE conversation_id = ?
			ORDER BY received_at, id
		`
	}
	rows, err := l.db.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("query chat log %s: %w", id, err)
	}
	defer rows.Close()

	seen := make([]Record, 0)
	records := make([]Record, 0)
	for rows.Next() {
		rec, err := scanSQLiteRecord(rows)
		if err != nil {
			return nil, err
		}
		if isDuplicateRecord(seen, rec) {
			continue
		}
		if rec.MsgID != "" {
			seen = append(seen, rec)
		}
		if !rec.ReceivedAt.Before(since) {
			records = append(records, rec)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat log %s: %w", id, err)
	}
	return records, nil
}

func scanSQLiteRecord(row interface{ Scan(dest ...any) error }) (Record, error) {
	var rec Record
	var receivedAt string
	var src, srcType, via, dst, msgID, sequenceID sql.NullString
	var rssi, snr sql.NullInt64
	var direction, deliveryStatus sql.NullString
	var ackStatus, ackReceivedAt, ackSrc, ackSrcType, ackVia sql.NullString
	var ackRSSI, ackSNR sql.NullInt64
	if err := row.Scan(&receivedAt, &src, &srcType, &via, &dst, &msgID, &sequenceID, &rec.Msg, &rssi, &snr, &direction, &deliveryStatus, &ackStatus, &ackReceivedAt, &ackSrc, &ackSrcType, &ackRSSI, &ackSNR, &ackVia); err != nil {
		return Record{}, fmt.Errorf("scan chat record: %w", err)
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, receivedAt)
	if err != nil {
		return Record{}, fmt.Errorf("parse chat record timestamp: %w", err)
	}
	rec.ReceivedAt = parsedAt
	rec.Src = src.String
	rec.SrcType = srcType.String
	if via.Valid && via.String != "" {
		if err := json.Unmarshal([]byte(via.String), &rec.Via); err != nil {
			return Record{}, fmt.Errorf("parse chat via: %w", err)
		}
	}
	rec.Dst = dst.String
	rec.MsgID = msgID.String
	rec.SequenceID = sequenceID.String
	if rssi.Valid {
		rec.RSSI = int(rssi.Int64)
	}
	if snr.Valid {
		rec.SNR = int(snr.Int64)
	}
	rec.Direction = direction.String
	rec.DeliveryStatus = deliveryStatus.String
	rec.AckStatus = ackStatus.String
	if ackReceivedAt.Valid {
		parsedAckAt, err := time.Parse(time.RFC3339Nano, ackReceivedAt.String)
		if err != nil {
			return Record{}, fmt.Errorf("parse chat ack timestamp: %w", err)
		}
		rec.AckReceivedAt = parsedAckAt
	}
	rec.AckSrc = ackSrc.String
	rec.AckSrcType = ackSrcType.String
	if ackRSSI.Valid {
		rec.AckRSSI = int(ackRSSI.Int64)
	}
	if ackSNR.Valid {
		rec.AckSNR = int(ackSNR.Int64)
	}
	if ackVia.Valid && ackVia.String != "" {
		if err := json.Unmarshal([]byte(ackVia.String), &rec.AckVia); err != nil {
			return Record{}, fmt.Errorf("parse chat ack via: %w", err)
		}
	}
	return rec, nil
}

// ReadSince returns records from the conversation JSONL file with ReceivedAt >= since,
// sorted ascending by ReceivedAt. Malformed lines are skipped with a warning.
func (l *Logger) ReadSince(id string, since time.Time) ([]Record, error) {
	if !validID.MatchString(id) {
		return nil, ErrInvalidID
	}
	return l.readSinceSQLite(id, since)
}
