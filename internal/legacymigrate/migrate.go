// Package legacymigrate performs one-time startup migration of legacy data files
// to the storage layout introduced in the basecall-namespaced DM redesign.
//
// DEPRECATED: This package is scheduled for removal once all deployments have
// migrated. Delete it (and its call in cmd/gomeshcomd/main.go) in a future
// release after the migration window has closed.
package legacymigrate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// sanitize mirrors chatlog.Sanitize to keep this package self-contained and
// deletable without touching other packages.
func sanitize(s string) string {
	return strings.ToUpper(unsafeChars.ReplaceAllString(s, "_"))
}

// baseCall strips a numeric SSID suffix from a callsign.
// IU5PMP-1 → IU5PMP, IU5PMP-10 → IU5PMP, IU5PMP → IU5PMP.
func baseCall(callsign string) string {
	if i := strings.LastIndex(callsign, "-"); i >= 0 {
		suffix := callsign[i+1:]
		if isNumeric(suffix) {
			return callsign[:i]
		}
	}
	return callsign
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isLegacyDMID returns true for DM ids like DM_IK5FCK-10 where the part after
// "DM_" contains no underscore. Migrated ids (DM_IU5PMP_IK5FCK-10 or
// DM_IU5PMP-1_IK5FCK-10) contain an underscore in that segment.
func isLegacyDMID(id string) bool {
	if !strings.HasPrefix(id, "DM_") {
		return false
	}
	return !strings.Contains(id[3:], "_")
}

// statusEntry mirrors chatstatus.Entry. Kept local to avoid importing the
// chatstatus package so this migration file is fully self-contained.
type statusEntry struct {
	LastMsgReceived time.Time `json:"lastMsgReceived"`
	LastRead        time.Time `json:"lastRead"`
	UnreadCount     int       `json:"unreadCount"`
	LastMsg         string    `json:"lastMsg,omitempty"`
}

// Run executes all legacy data migrations. It is idempotent: already-migrated
// data is left unchanged. An empty myCall causes DM and msg_idx migration to
// be skipped with a warning — only the config-file move is attempted.
func Run(dataDir, chatPath, myCall string) error {
	myCall = strings.ToUpper(strings.TrimSpace(myCall))

	if err := migrateChannelShow(dataDir); err != nil {
		return fmt.Errorf("legacymigrate channel_show: %w", err)
	}

	if myCall == "" {
		slog.Warn("legacymigrate: myCall empty, skipping DM file and msg_idx migration")
		return nil
	}

	if err := migrateDMFiles(chatPath, myCall); err != nil {
		return fmt.Errorf("legacymigrate DM files: %w", err)
	}

	if err := migrateMsgIdx(chatPath, myCall); err != nil {
		return fmt.Errorf("legacymigrate msg_idx: %w", err)
	}

	return nil
}

// migrateChannelShow moves the legacy data/channel_show.json to
// data/configs/channel_show.json when only the legacy file exists.
func migrateChannelShow(dataDir string) error {
	legacyPath := filepath.Join(dataDir, "channel_show.json")
	newPath := filepath.Join(dataDir, "configs", "channel_show.json")

	if !fileExists(legacyPath) {
		return nil // nothing to migrate
	}
	if fileExists(newPath) {
		slog.Warn("legacymigrate: both legacy and new channel_show.json exist, skipping move",
			"legacy", legacyPath, "new", newPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "configs"), 0o755); err != nil {
		return fmt.Errorf("create configs dir: %w", err)
	}
	if err := os.Rename(legacyPath, newPath); err != nil {
		return fmt.Errorf("rename channel_show.json: %w", err)
	}
	slog.Info("legacymigrate: moved channel_show.json to configs/",
		"src", legacyPath, "dst", newPath)
	return nil
}

// migrateDMFiles renames legacy DM_<peer>.jsonl files to the new
// DM_<basecall(myCall)>_<peer>.jsonl format. When the target already exists,
// records are merged (dedup by msg_id) rather than overwriting.
func migrateDMFiles(chatPath, myCall string) error {
	entries, err := os.ReadDir(chatPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read chat dir: %w", err)
	}

	myBase := sanitize(baseCall(myCall))

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(name, ".jsonl")
		if !isLegacyDMID(id) {
			continue
		}
		peer := id[3:] // strip "DM_"
		newID := "DM_" + myBase + "_" + peer
		srcPath := filepath.Join(chatPath, name)
		dstPath := filepath.Join(chatPath, newID+".jsonl")

		if !fileExists(dstPath) {
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("rename %s: %w", name, err)
			}
			slog.Info("legacymigrate: renamed DM file", "from", name, "to", newID+".jsonl")
		} else {
			if err := mergeDMFiles(srcPath, dstPath); err != nil {
				return fmt.Errorf("merge %s into %s: %w", name, newID+".jsonl", err)
			}
			slog.Info("legacymigrate: merged DM file", "src", name, "dst", newID+".jsonl")
		}
	}
	return nil
}

// jsonRecord is a raw JSON object used to copy records without knowing their
// concrete type, preserving all fields even if this package's statusEntry type
// does not define them.
type jsonRecord map[string]any

// mergeDMFiles merges records from srcPath into dstPath, deduplicating by
// msg_id. Writes to dstPath atomically and removes srcPath on success.
func mergeDMFiles(srcPath, dstPath string) error {
	srcRecords, err := readJSONLines(srcPath)
	if err != nil {
		return fmt.Errorf("read src %s: %w", srcPath, err)
	}
	dstRecords, err := readJSONLines(dstPath)
	if err != nil {
		return fmt.Errorf("read dst %s: %w", dstPath, err)
	}

	seen := make(map[string]struct{}, len(dstRecords))
	merged := make([]jsonRecord, 0, len(dstRecords)+len(srcRecords))

	for _, rec := range dstRecords {
		if id, _ := rec["msg_id"].(string); id != "" {
			seen[id] = struct{}{}
		}
		merged = append(merged, rec)
	}
	for _, rec := range srcRecords {
		id, _ := rec["msg_id"].(string)
		if id != "" {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
		}
		merged = append(merged, rec)
	}

	tmp := dstPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	enc := json.NewEncoder(f)
	for _, rec := range merged {
		if encErr := enc.Encode(rec); encErr != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("encode: %w", encErr)
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, dstPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(srcPath)
}

// readJSONLines reads JSONL records from path. Malformed lines are skipped with
// a warning. A missing file returns an empty slice.
func readJSONLines(path string) ([]jsonRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []jsonRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec jsonRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			slog.Warn("legacymigrate: skipping malformed record", "path", path, "error", err)
			continue
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

// migrateMsgIdx rewrites legacy msg_idx.json DM keys from DM_<peer> to the
// full-SSID form DM_<myCall>_<peer>. On key collision the entry with the more
// recent LastMsgReceived is kept.
func migrateMsgIdx(chatPath, myCall string) error {
	idxPath := filepath.Join(chatPath, "msg_idx.json")

	f, err := os.Open(idxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open msg_idx.json: %w", err)
	}

	var raw map[string]*statusEntry
	if decErr := json.NewDecoder(f).Decode(&raw); decErr != nil {
		f.Close()
		return fmt.Errorf("decode msg_idx.json: %w", decErr)
	}
	f.Close()

	myCallKey := sanitize(myCall) // IU5PMP-1 → IU5PMP-1 (already safe chars)
	out := make(map[string]*statusEntry, len(raw))

	for key, entry := range raw {
		if !isLegacyDMID(key) {
			// Already migrated or P_*: keep as-is.
			out[key] = entry
			continue
		}
		peer := key[3:] // strip "DM_"
		newKey := "DM_" + myCallKey + "_" + peer
		if existing, ok := out[newKey]; ok {
			// Collision: keep the entry with the more recent received timestamp.
			if entry.LastMsgReceived.After(existing.LastMsgReceived) {
				out[newKey] = entry
			}
			slog.Warn("legacymigrate: msg_idx key collision, kept most recent", "key", newKey)
		} else {
			out[newKey] = entry
		}
	}

	return writeMsgIdx(idxPath, out)
}

// writeMsgIdx atomically writes entries to path using a temp-file + rename.
func writeMsgIdx(path string, entries map[string]*statusEntry) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp msg_idx: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(entries); encErr != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode msg_idx: %w", encErr)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
