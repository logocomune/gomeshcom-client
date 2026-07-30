package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltInSerialDefaults(t *testing.T) {
	cfg := builtInDefaultConfig()
	if cfg.TransportMode != TransportUDP {
		t.Fatalf("TransportMode = %q, want %q", cfg.TransportMode, TransportUDP)
	}
	want := Serial{
		Baud:             115200,
		DataBits:         8,
		Parity:           "none",
		StopBits:         1,
		FlowControl:      "none",
		DTR:              false,
		RTS:              false,
		ReadTimeout:      time.Second,
		ReconnectInitial: time.Second,
		ReconnectMax:     30 * time.Second,
		StableResetAfter: 30 * time.Second,
		MaxRecordBytes:   65536,
	}
	if cfg.Serial != want {
		t.Fatalf("Serial = %+v, want %+v", cfg.Serial, want)
	}
}

func TestValidateTransport(t *testing.T) {
	validSerial := func() Config {
		cfg := builtInDefaultConfig()
		cfg.MyCall = "QQ1ABC-1"
		cfg.TransportMode = TransportSerial
		cfg.Serial.Device = "/dev/ttyUSB0"
		return cfg
	}
	mutateSerial := func(change func(*Serial)) Config {
		cfg := validSerial()
		change(&cfg.Serial)
		return cfg
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "udp remains default",
			cfg: func() Config {
				cfg := builtInDefaultConfig()
				cfg.MyCall = "QQ1ABC-1"
				cfg.TransportMode = ""
				return cfg
			}(),
		},
		{
			name: "serial valid",
			cfg:  validSerial(),
		},
		{
			name: "serial ignores inactive udp addresses",
			cfg: func() Config {
				cfg := validSerial()
				cfg.UDPListenAddr = "invalid"
				cfg.NodeAddr = "invalid"
				return cfg
			}(),
		},
		{
			name: "unknown mode",
			cfg: func() Config {
				cfg := validSerial()
				cfg.TransportMode = "bluetooth"
				return cfg
			}(),
			wantErr: "transport mode",
		},
		{
			name:    "missing device",
			cfg:     mutateSerial(func(serial *Serial) { serial.Device = "" }),
			wantErr: "serial device",
		},
		{
			name:    "invalid baud",
			cfg:     mutateSerial(func(serial *Serial) { serial.Baud = 0 }),
			wantErr: "serial baud",
		},
		{
			name:    "invalid data bits",
			cfg:     mutateSerial(func(serial *Serial) { serial.DataBits = 9 }),
			wantErr: "serial data bits",
		},
		{
			name:    "invalid parity",
			cfg:     mutateSerial(func(serial *Serial) { serial.Parity = "invalid" }),
			wantErr: "serial parity",
		},
		{
			name:    "invalid stop bits",
			cfg:     mutateSerial(func(serial *Serial) { serial.StopBits = 3 }),
			wantErr: "serial stop bits",
		},
		{
			name:    "unsupported flow control",
			cfg:     mutateSerial(func(serial *Serial) { serial.FlowControl = "hardware" }),
			wantErr: "serial flow control",
		},
		{
			name:    "invalid read timeout",
			cfg:     mutateSerial(func(serial *Serial) { serial.ReadTimeout = 0 }),
			wantErr: "serial read timeout",
		},
		{
			name:    "invalid initial reconnect",
			cfg:     mutateSerial(func(serial *Serial) { serial.ReconnectInitial = 0 }),
			wantErr: "serial reconnect initial",
		},
		{
			name:    "invalid maximum reconnect",
			cfg:     mutateSerial(func(serial *Serial) { serial.ReconnectMax = 0 }),
			wantErr: "serial reconnect maximum",
		},
		{
			name: "initial reconnect exceeds maximum",
			cfg: mutateSerial(func(serial *Serial) {
				serial.ReconnectInitial = 31 * time.Second
				serial.ReconnectMax = 30 * time.Second
			}),
			wantErr: "must not exceed",
		},
		{
			name:    "invalid stable reset duration",
			cfg:     mutateSerial(func(serial *Serial) { serial.StableResetAfter = 0 }),
			wantErr: "serial stable reset",
		},
		{
			name:    "invalid record size",
			cfg:     mutateSerial(func(serial *Serial) { serial.MaxRecordBytes = 0 }),
			wantErr: "serial maximum record size",
		},
		{
			name: "record size exceeds safe limit",
			cfg: mutateSerial(func(serial *Serial) {
				serial.MaxRecordBytes = 1<<20 + 1
			}),
			wantErr: "serial maximum record size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeSerialConfiguration(t *testing.T) {
	cfg := normalize(Config{
		TransportMode: " SERIAL ",
		MyCall:        "qq1abc-1",
		Serial: Serial{
			Device:      " /dev/ttyUSB0 ",
			Parity:      " EVEN ",
			FlowControl: " NONE ",
		},
	})
	if cfg.TransportMode != TransportSerial {
		t.Fatalf("TransportMode = %q, want serial", cfg.TransportMode)
	}
	if cfg.MyCall != "QQ1ABC-1" {
		t.Fatalf("MyCall = %q, want QQ1ABC-1", cfg.MyCall)
	}
	if cfg.Serial.Device != "/dev/ttyUSB0" ||
		cfg.Serial.Parity != "even" ||
		cfg.Serial.FlowControl != "none" {
		t.Fatalf("Serial = %+v", cfg.Serial)
	}
}

func TestMergeSerialTomlAndEnvironmentPrecedence(t *testing.T) {
	cfg := builtInDefaultConfig()
	mode := TransportSerial
	device := "/dev/ttyUSB9"
	baud := 57600
	dataBits := 7
	parity := "even"
	stopBits := 2
	flowControl := "none"
	dtr := true
	rts := true
	readTimeout := tomlDuration{Duration: 2 * time.Second}
	reconnectInitial := tomlDuration{Duration: 3 * time.Second}
	reconnectMax := tomlDuration{Duration: 40 * time.Second}
	stableResetAfter := tomlDuration{Duration: time.Minute}
	maxRecordBytes := 8192
	tf := &tomlFile{
		TransportMode: &mode,
		Serial: &tomlSerial{
			Device:           &device,
			Baud:             &baud,
			DataBits:         &dataBits,
			Parity:           &parity,
			StopBits:         &stopBits,
			FlowControl:      &flowControl,
			DTR:              &dtr,
			RTS:              &rts,
			ReadTimeout:      &readTimeout,
			ReconnectInitial: &reconnectInitial,
			ReconnectMax:     &reconnectMax,
			StableResetAfter: &stableResetAfter,
			MaxRecordBytes:   &maxRecordBytes,
		},
	}

	mergeToml(&cfg, tf, EnvOverrides{
		"SERIAL_DEVICE": true,
		"SERIAL_DTR":    true,
	})

	if cfg.TransportMode != TransportSerial {
		t.Fatalf("TransportMode = %q, want serial", cfg.TransportMode)
	}
	if cfg.Serial.Device != "" {
		t.Fatalf("Serial.Device = %q, want env/default value", cfg.Serial.Device)
	}
	if cfg.Serial.DTR {
		t.Fatal("Serial.DTR = true, want env/default false")
	}
	if cfg.Serial.Baud != 57600 ||
		cfg.Serial.DataBits != 7 ||
		cfg.Serial.Parity != "even" ||
		cfg.Serial.StopBits != 2 ||
		cfg.Serial.FlowControl != "none" ||
		!cfg.Serial.RTS ||
		cfg.Serial.ReadTimeout != 2*time.Second ||
		cfg.Serial.ReconnectInitial != 3*time.Second ||
		cfg.Serial.ReconnectMax != 40*time.Second ||
		cfg.Serial.StableResetAfter != time.Minute ||
		cfg.Serial.MaxRecordBytes != 8192 {
		t.Fatalf("Serial = %+v", cfg.Serial)
	}
}

func TestSerialTomlRoundTrip(t *testing.T) {
	cfg := builtInDefaultConfig()
	cfg.TransportMode = TransportSerial
	cfg.Serial.Device = "COM12"
	cfg.Serial.DTR = true

	path := filepath.Join(t.TempDir(), "gomeshcomd.toml")
	if err := WriteToml(path, cfg); err != nil {
		t.Fatalf("WriteToml() error = %v", err)
	}
	tf, err := loadTomlFile(path)
	if err != nil {
		t.Fatalf("loadTomlFile() error = %v", err)
	}
	loaded := builtInDefaultConfig()
	mergeToml(&loaded, tf, EnvOverrides{})
	loaded = normalize(loaded)

	if loaded.TransportMode != cfg.TransportMode || loaded.Serial != cfg.Serial {
		t.Fatalf("round trip = mode %q serial %+v, want mode %q serial %+v", loaded.TransportMode, loaded.Serial, cfg.TransportMode, cfg.Serial)
	}
}

func TestLoadLegacyTomlDefaultsToUDP(t *testing.T) {
	loadTestSetup(t)
	dataDir := t.TempDir()
	t.Setenv("GOMESHCOM_DATA_DIR", dataDir)

	configDir := filepath.Join(dataDir, "configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `
udp_listen_addr = "127.0.0.1:1799"
node_addr = ""
my_call = "QQ1ABC-1"
`
	if err := os.WriteFile(filepath.Join(configDir, "gomeshcomd.toml"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, _, _, err := Load("test")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TransportMode != TransportUDP {
		t.Fatalf("TransportMode = %q, want udp", cfg.TransportMode)
	}
	if cfg.UDPListenAddr != "127.0.0.1:1799" {
		t.Fatalf("UDPListenAddr = %q", cfg.UDPListenAddr)
	}
}

func TestLoadSerialEnvironment(t *testing.T) {
	loadTestSetup(t)
	dataDir := t.TempDir()
	t.Setenv("GOMESHCOM_DATA_DIR", dataDir)
	t.Setenv("GOMESHCOM_TRANSPORT_MODE", "serial")
	t.Setenv("GOMESHCOM_SERIAL_DEVICE", "COM7")
	t.Setenv("GOMESHCOM_SERIAL_DTR", "true")
	t.Setenv("GOMESHCOM_SERIAL_RTS", "false")

	cfg, env, _, err := Load("test")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TransportMode != TransportSerial ||
		cfg.Serial.Device != "COM7" ||
		!cfg.Serial.DTR ||
		cfg.Serial.RTS {
		t.Fatalf("loaded config = mode %q serial %+v", cfg.TransportMode, cfg.Serial)
	}
	for _, suffix := range []string{"TRANSPORT_MODE", "SERIAL_DEVICE", "SERIAL_DTR", "SERIAL_RTS"} {
		if !env[suffix] {
			t.Fatalf("env[%q] = false", suffix)
		}
	}
}
