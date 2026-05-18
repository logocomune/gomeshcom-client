package config

import (
	"os"
	"testing"
)

func TestLoadAllowsEmptyNodeAddr(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"gomeshcomd"}
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	t.Setenv("GOMESHCOM_NODE_ADDR", "")
	t.Setenv("GOMESHCOM_MY_CALL", "QQ1ABC-1")
	t.Setenv("GOMESHCOM_HTTP_ADDR", "127.0.0.1:8080")
	t.Setenv("GOMESHCOM_UDP_LISTEN_ADDR", "0.0.0.0:1799")
	t.Setenv("GOMESHCOM_DATA_DIR", "./data")
	t.Setenv("GOMESHCOM_MAX_MESSAGE_LENGTH", "149")
	t.Setenv("GOMESHCOM_LOG_LEVEL", "info")

	cfg, _, err := Load("test")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.NodeAddr != "" {
		t.Fatalf("NodeAddr = %q, want empty", cfg.NodeAddr)
	}
}

func TestLoadRequestLogEnabled(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"gomeshcomd"}
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	t.Setenv("GOMESHCOM_NODE_ADDR", "")
	t.Setenv("GOMESHCOM_MY_CALL", "QQ1ABC-1")
	t.Setenv("GOMESHCOM_HTTP_ADDR", "127.0.0.1:8080")
	t.Setenv("GOMESHCOM_UDP_LISTEN_ADDR", "0.0.0.0:1799")
	t.Setenv("GOMESHCOM_DATA_DIR", "./data")
	t.Setenv("GOMESHCOM_MAX_MESSAGE_LENGTH", "149")
	t.Setenv("GOMESHCOM_LOG_LEVEL", "info")
	t.Setenv("GOMESHCOM_REQUEST_LOG_ENABLED", "true")

	cfg, _, err := Load("test")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.RequestLog.Enabled {
		t.Fatal("RequestLog.Enabled = false, want true")
	}
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		cfg     Config
		wantErr bool
	}{
		"valid": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-12",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ReceiveLog: ReceiveLog{
					Enabled:       true,
					Path:          "./data/raw",
					RetentionDays: 365,
					ReplayWindow:  10,
				},
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9, // 24h in ns
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
		},
		"chat log missing path": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"chat log zero history window": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"chat log max less than history": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    48 * 60 * 60 * 1e9,
					MaxHistoryWindow: 24 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"invalid http addr": {
			cfg: Config{
				HTTPAddr:         "bad address",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"invalid udp listen addr": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "bad address",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"invalid node addr": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "bad address",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"invalid max length": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 0,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"invalid send delay": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				SendDelay:        -1,
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"invalid log level": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "trace",
			},
			wantErr: true,
		},
		"missing data dir": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"enabled receive log missing path": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ReceiveLog: ReceiveLog{
					Enabled: true,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"enabled receive log invalid replay window": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ReceiveLog: ReceiveLog{
					Enabled:      true,
					Path:         "./data/raw",
					ReplayWindow: -1,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"missing my call": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"missing ssid": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"invalid my call": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "BAD_CALL",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
		"enabled receive log invalid retention days": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ReceiveLog: ReceiveLog{
					Enabled:       true,
					Path:          "./data/raw",
					RetentionDays: -1,
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"negative send dedup TTL": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "192.168.0.2:1799",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				Send:     Send{DedupTTL: -1},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"auth enabled with username and password": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				Auth: Auth{
					Username:   "admin",
					Password:   "secret",
					SessionTTL: 24 * 60 * 60 * 1e9,
					CookieName: "meshcom_session",
				},
				LogLevel: "info",
			},
		},
		"auth username without password": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				Auth: Auth{
					Username:   "admin",
					SessionTTL: 24 * 60 * 60 * 1e9,
					CookieName: "meshcom_session",
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"auth password without username": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				Auth: Auth{
					Password:   "secret",
					SessionTTL: 24 * 60 * 60 * 1e9,
					CookieName: "meshcom_session",
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"auth enabled with zero session ttl": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				Auth: Auth{
					Username:   "admin",
					Password:   "secret",
					SessionTTL: 0,
					CookieName: "meshcom_session",
				},
				LogLevel: "info",
			},
			wantErr: true,
		},
		"empty node addr is valid (auto-detect)": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				ChatLog: ChatLog{
					Path:             "./data/chat",
					HistoryWindow:    24 * 60 * 60 * 1e9,
					MaxHistoryWindow: 720 * 60 * 60 * 1e9,
				},
				LogLevel: "info",
			},
			wantErr: false,
		},
		"non-empty invalid node addr fails": {
			cfg: Config{
				HTTPAddr:         "127.0.0.1:8080",
				UDPListenAddr:    "0.0.0.0:1799",
				NodeAddr:         "not-an-addr",
				MyCall:           "QQ1ABC-1",
				DataDir:          "./data",
				MaxMessageLength: 149,
				LogLevel:         "info",
			},
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := Validate(test.cfg)
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateForwardTargets(t *testing.T) {
	base := func() Config {
		return Config{
			HTTPAddr:         "127.0.0.1:8080",
			UDPListenAddr:    "0.0.0.0:1799",
			MyCall:           "QQ1ABC-1",
			DataDir:          "./data",
			MaxMessageLength: 149,
			ChatLog: ChatLog{
				Path:             "./data/chat",
				HistoryWindow:    24 * 60 * 60 * 1e9,
				MaxHistoryWindow: 720 * 60 * 60 * 1e9,
			},
			LogLevel: "info",
		}
	}

	tests := map[string]struct {
		targets string
		wantErr bool
	}{
		"empty string is valid (disabled)": {targets: "", wantErr: false},
		"single valid target":              {targets: "127.0.0.1:9999", wantErr: false},
		"two valid targets":                {targets: "127.0.0.1:9001,127.0.0.1:9002", wantErr: false},
		"whitespace around entries":        {targets: " 127.0.0.1:9001 , 127.0.0.1:9002 ", wantErr: false},
		"duplicates are collapsed":         {targets: "127.0.0.1:9001,127.0.0.1:9001", wantErr: false},
		"invalid host:port":                {targets: "not-a-host:notaport", wantErr: true},
		"missing port":                     {targets: "127.0.0.1", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := base()
			cfg.Forward.Targets = tc.targets
			err := Validate(cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestParseForwardTargets(t *testing.T) {
	tests := map[string]struct {
		input   string
		want    []string
		wantErr bool
	}{
		"empty":              {input: "", want: nil},
		"single":             {input: "127.0.0.1:9000", want: []string{"127.0.0.1:9000"}},
		"two targets":        {input: "127.0.0.1:9000,127.0.0.1:9001", want: []string{"127.0.0.1:9000", "127.0.0.1:9001"}},
		"whitespace trimmed": {input: " 127.0.0.1:9000 , 127.0.0.1:9001 ", want: []string{"127.0.0.1:9000", "127.0.0.1:9001"}},
		"duplicates removed": {input: "127.0.0.1:9000,127.0.0.1:9000", want: []string{"127.0.0.1:9000"}},
		"empty csv entries":  {input: "127.0.0.1:9000,,127.0.0.1:9001", want: []string{"127.0.0.1:9000", "127.0.0.1:9001"}},
		"invalid addr":       {input: "bad:notaport", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ParseForwardTargets(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseForwardTargets() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr {
				if len(got) != len(tc.want) {
					t.Fatalf("len = %d, want %d; got %v", len(got), len(tc.want), got)
				}
				for i, v := range tc.want {
					if got[i] != v {
						t.Fatalf("got[%d] = %q, want %q", i, got[i], v)
					}
				}
			}
		})
	}
}

func TestNormalizeUppercasesMyCall(t *testing.T) {
	cfg := normalize(Config{MyCall: "qq5akt-10"})

	if cfg.MyCall != "QQ5AKT-10" {
		t.Fatalf("MyCall = %q, want QQ5AKT-10", cfg.MyCall)
	}
}
