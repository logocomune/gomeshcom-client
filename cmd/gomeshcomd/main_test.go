package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/config"
)

func TestEnsureDataDirs(t *testing.T) {
	dataDir := t.TempDir()

	if err := ensureDataDirs(dataDir); err != nil {
		t.Fatalf("ensureDataDirs() error = %v", err)
	}

	for _, dir := range []string{"raw", "nodes"} {
		info, err := os.Stat(filepath.Join(dataDir, dir))
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not directory", dir)
		}
	}

	if _, err := os.Stat(filepath.Join(dataDir, "messages")); !os.IsNotExist(err) {
		t.Fatalf("messages stat error = %v, want not exist", err)
	}
}

func TestStartupBanner(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "0.0.0.0:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "192.168.0.2:1799",
		MyCall:        "QQ1ABC-1",
	}

	banner := startupBanner(cfg)
	wants := []string{
		"GOMESHCOMD",
		"MeshCom UDP Link Terminal",
		"STATUS   READY",
		"VERSION  dev",
		"MYCALL   QQ1ABC-1",
		"NODE     192.168.0.2:1799",
		"HELP     gomeshcomd --help",
		"UDP RX   0.0.0.0:1799",
		"WEB UI   http://127.0.0.1:8080",
	}

	for _, want := range wants {
		if !strings.Contains(banner, want) {
			t.Fatalf("startup banner missing %q:\n%s", want, banner)
		}
	}
}

func TestStartupBannerAutoDetectNode(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "",
		MyCall:        "QQ1ABC-1",
	}

	banner := startupBanner(cfg)
	if !strings.Contains(banner, "NODE     (auto-detect from incoming UDP)") {
		t.Fatalf("startup banner missing auto-detect message:\n%s", banner)
	}
}

func TestStartupBannerRowsStayBoxed(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "192.168.0.2:1799",
	}

	rows := strings.Split(strings.TrimSuffix(startupBanner(cfg), "\n"), "\n")
	if len(rows) == 0 {
		t.Fatal("startup banner is empty")
	}

	wantLength := len(rows[0])
	for _, row := range rows {
		if len(row) != wantLength {
			t.Fatalf("row length = %d, want %d: %q", len(row), wantLength, row)
		}
	}
}

func TestStartupBannerShowsUnsetMyCallHint(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "192.168.0.2:1799",
	}

	banner := startupBanner(cfg)
	wants := []string{
		"MYCALL   (unset)",
		"DMs      hidden until MyCall set",
		"MSG      node addr required",
	}

	for _, want := range wants {
		if !strings.Contains(banner, want) {
			t.Fatalf("startup banner missing %q:\n%s", want, banner)
		}
	}
}

func TestWebInterfaceURL(t *testing.T) {
	tests := []struct {
		name     string
		httpAddr string
		want     string
	}{
		{
			name:     "loopback",
			httpAddr: "127.0.0.1:8080",
			want:     "http://127.0.0.1:8080",
		},
		{
			name:     "all interfaces",
			httpAddr: "0.0.0.0:8080",
			want:     "http://127.0.0.1:8080",
		},
		{
			name:     "empty host",
			httpAddr: ":8080",
			want:     "http://127.0.0.1:8080",
		},
		{
			name:     "ipv6 all interfaces",
			httpAddr: "[::]:8080",
			want:     "http://127.0.0.1:8080",
		},
		{
			name:     "ipv6 loopback",
			httpAddr: "[::1]:8080",
			want:     "http://[::1]:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webInterfaceURL(tt.httpAddr); got != tt.want {
				t.Fatalf("webInterfaceURL(%q) = %q, want %q", tt.httpAddr, got, tt.want)
			}
		})
	}
}
