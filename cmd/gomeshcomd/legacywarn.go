package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
)

// legacyDataProbes lists the locations written by the removed file-based
// persistence layer, relative to the data directory. Globs are allowed.
var legacyDataProbes = []string{
	filepath.Join("raw", "received.*.jsonl"),
	filepath.Join("chat", "*.jsonl"),
	filepath.Join("chat", "msg_idx.json"),
	filepath.Join("stats", "stats.json"),
	filepath.Join("stats", "dm_stats.json"),
	filepath.Join("nodes", "positions.json"),
	filepath.Join("configs", "channel_show.json"),
	filepath.Join("configs", "station.json"),
	"channel_show.json",
	"http-sessions.json",
}

// findLegacyDataFiles returns the files left behind by the removed file-based
// persistence layer. Missing directories and unreadable entries are ignored:
// this is a best-effort startup hint, never a failure.
func findLegacyDataFiles(dataDir string) []string {
	var found []string
	for _, probe := range legacyDataProbes {
		matches, err := filepath.Glob(filepath.Join(dataDir, probe))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				found = append(found, match)
			}
		}
	}
	sort.Strings(found)
	return found
}

// warnLegacyDataFiles logs one warning when data from the removed file-based
// persistence layer is still on disk. Those files are neither imported nor
// deleted: they must be imported with a pre-upgrade binary.
func warnLegacyDataFiles(dataDir string, logger *slog.Logger) {
	files := findLegacyDataFiles(dataDir)
	if len(files) == 0 {
		return
	}

	logger.Warn("legacy data files found; file-based import was removed, these files are not imported",
		"data_dir", dataDir,
		"files", files,
	)
}
