package main

import (
	"strings"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/config"
)

func TestStartupBanner(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "0.0.0.0:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "192.168.0.2:1799",
		MyCall:        "QQ1ABC-1",
	}

	banner := startupBanner(cfg, cfg.MyCall)
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

	banner := startupBanner(cfg, cfg.MyCall)
	if !strings.Contains(banner, "NODE     (auto-detect from incoming UDP)") {
		t.Fatalf("startup banner missing auto-detect message:\n%s", banner)
	}
}

func TestStartupBannerSerialTransport(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		TransportMode: config.TransportSerial,
		MyCall:        "QQ1ABC-1",
		Serial: config.Serial{
			Device: "/dev/ttyUSB0",
		},
	}

	banner := startupBanner(cfg, cfg.MyCall)
	for _, want := range []string{
		"MeshCom SERIAL Link Terminal",
		"NODE     /dev/ttyUSB0",
		"SERIAL   /dev/ttyUSB0",
	} {
		if !strings.Contains(banner, want) {
			t.Fatalf("startup banner missing %q:\n%s", want, banner)
		}
	}
	if strings.Contains(banner, "UDP RX") {
		t.Fatalf("serial startup banner contains UDP row:\n%s", banner)
	}
}

func TestStartupBannerNetConsoleTransport(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		TransportMode: config.TransportNetConsole,
		MyCall:        "QQ1ABC-1",
		NetConsole: config.NetConsole{
			Address: "meshcom.local:1799",
		},
	}

	banner := startupBanner(cfg, cfg.MyCall)
	for _, want := range []string{
		"MeshCom NETCONSOLE Link Terminal",
		"NODE     meshcom.local:1799",
		"NETCON   meshcom.local:1799",
	} {
		if !strings.Contains(banner, want) {
			t.Fatalf("startup banner missing %q:\n%s", want, banner)
		}
	}
	if strings.Contains(banner, "UDP RX") {
		t.Fatalf("NETConsole startup banner contains UDP row:\n%s", banner)
	}
}

func TestStartupBannerRowsStayBoxed(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:      "127.0.0.1:8080",
		UDPListenAddr: "0.0.0.0:1799",
		NodeAddr:      "192.168.0.2:1799",
	}

	rows := strings.Split(strings.TrimSuffix(startupBanner(cfg, cfg.MyCall), "\n"), "\n")
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

	banner := startupBanner(cfg, cfg.MyCall)
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
