package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindLegacyDataFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		dirs  []string
		want  []string
	}{
		{
			name: "missing data dir yields nothing",
		},
		{
			name:  "empty data dir yields nothing",
			files: []string{"gomeshcom.db"},
			want:  nil,
		},
		{
			name: "unrelated files are ignored",
			files: []string{
				"configs/gomeshcomd.toml",
				"notes.txt",
				"chat/summary.md",
			},
			want: nil,
		},
		{
			name: "receive log and chat files are reported",
			files: []string{
				"raw/received.20250101.jsonl",
				"raw/received.20250102.jsonl",
				"chat/P_broadcast.jsonl",
				"chat/msg_idx.json",
			},
			want: []string{
				"chat/P_broadcast.jsonl",
				"chat/msg_idx.json",
				"raw/received.20250101.jsonl",
				"raw/received.20250102.jsonl",
			},
		},
		{
			name: "single-file stores are reported",
			files: []string{
				"stats/stats.json",
				"stats/dm_stats.json",
				"nodes/positions.json",
				"configs/channel_show.json",
				"configs/station.json",
				"channel_show.json",
				"http-sessions.json",
			},
			want: []string{
				"channel_show.json",
				"configs/channel_show.json",
				"configs/station.json",
				"http-sessions.json",
				"nodes/positions.json",
				"stats/dm_stats.json",
				"stats/stats.json",
			},
		},
		{
			name: "directories matching legacy names are skipped",
			dirs: []string{"http-sessions.json", "chat"},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()

			for _, file := range tc.files {
				full := filepath.Join(dataDir, file)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatalf("mkdir for %s: %v", file, err)
				}
				if err := os.WriteFile(full, []byte("{}"), 0o644); err != nil {
					t.Fatalf("write %s: %v", file, err)
				}
			}
			for _, dir := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", dir, err)
				}
			}

			got := findLegacyDataFiles(dataDir)
			if len(got) != len(tc.want) {
				t.Fatalf("findLegacyDataFiles() = %v, want %v", got, tc.want)
			}
			for i, want := range tc.want {
				if got[i] != filepath.Join(dataDir, want) {
					t.Fatalf("findLegacyDataFiles()[%d] = %q, want %q", i, got[i], filepath.Join(dataDir, want))
				}
			}
		})
	}
}

func TestFindLegacyDataFilesMissingDirIsNotAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	if got := findLegacyDataFiles(missing); got != nil {
		t.Fatalf("findLegacyDataFiles() = %v, want nil", got)
	}
}

func TestWarnLegacyDataFiles(t *testing.T) {
	tests := []struct {
		name       string
		seed       []string
		wantLogged bool
	}{
		{name: "clean data dir logs nothing"},
		{name: "seeded data dir logs a warning", seed: []string{"chat/P_broadcast.jsonl"}, wantLogged: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			for _, file := range tc.seed {
				full := filepath.Join(dataDir, file)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatalf("mkdir for %s: %v", file, err)
				}
				if err := os.WriteFile(full, []byte("{}"), 0o644); err != nil {
					t.Fatalf("write %s: %v", file, err)
				}
			}

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

			warnLegacyDataFiles(dataDir, logger)

			logged := buf.Len() > 0
			if logged != tc.wantLogged {
				t.Fatalf("logged = %v (output %q), want %v", logged, buf.String(), tc.wantLogged)
			}
			if tc.wantLogged && !strings.Contains(buf.String(), filepath.Join(dataDir, tc.seed[0])) {
				t.Fatalf("warning does not name the file: %q", buf.String())
			}
		})
	}
}
