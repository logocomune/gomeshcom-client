package serialbridge

import (
	"errors"
	"testing"
	"time"

	goserial "go.bug.st/serial"
)

type fakeConfigurablePort struct {
	dtr        bool
	rts        bool
	timeout    time.Duration
	dtrError   error
	rtsError   error
	timeoutErr error
}

func (p *fakeConfigurablePort) Read([]byte) (int, error) {
	return 0, nil
}

func (p *fakeConfigurablePort) Write(data []byte) (int, error) {
	return len(data), nil
}

func (p *fakeConfigurablePort) Close() error {
	return nil
}

func (p *fakeConfigurablePort) SetDTR(value bool) error {
	p.dtr = value
	return p.dtrError
}

func (p *fakeConfigurablePort) SetRTS(value bool) error {
	p.rts = value
	return p.rtsError
}

func (p *fakeConfigurablePort) SetReadTimeout(timeout time.Duration) error {
	p.timeout = timeout
	return p.timeoutErr
}

func TestLibraryModeMapsConfiguration(t *testing.T) {
	parities := map[string]goserial.Parity{
		"none":  goserial.NoParity,
		"odd":   goserial.OddParity,
		"even":  goserial.EvenParity,
		"mark":  goserial.MarkParity,
		"space": goserial.SpaceParity,
	}
	for parity, wantParity := range parities {
		t.Run(parity, func(t *testing.T) {
			mode, err := libraryMode(PortConfig{
				Baud:     115200,
				DataBits: 8,
				Parity:   parity,
				StopBits: 1,
				DTR:      true,
				RTS:      false,
			})
			if err != nil {
				t.Fatalf("libraryMode() error = %v", err)
			}
			if mode.BaudRate != 115200 ||
				mode.DataBits != 8 ||
				mode.Parity != wantParity ||
				mode.StopBits != goserial.OneStopBit {
				t.Fatalf("mode = %+v", mode)
			}
			if mode.InitialStatusBits == nil ||
				!mode.InitialStatusBits.DTR ||
				mode.InitialStatusBits.RTS {
				t.Fatalf("InitialStatusBits = %+v", mode.InitialStatusBits)
			}
		})
	}

	mode, err := libraryMode(PortConfig{
		Baud:     9600,
		DataBits: 7,
		Parity:   "none",
		StopBits: 2,
	})
	if err != nil {
		t.Fatalf("libraryMode(two stop bits) error = %v", err)
	}
	if mode.StopBits != goserial.TwoStopBits {
		t.Fatalf("StopBits = %v, want TwoStopBits", mode.StopBits)
	}
}

func TestLibraryModeRejectsUnsupportedValues(t *testing.T) {
	tests := []PortConfig{
		{Parity: "invalid", StopBits: 1},
		{Parity: "none", StopBits: 3},
	}
	for _, config := range tests {
		if _, err := libraryMode(config); err == nil {
			t.Fatalf("libraryMode(%+v) error = nil", config)
		}
	}
}

func TestConfigureOpenedPortAppliesControlLinesAndTimeout(t *testing.T) {
	port := &fakeConfigurablePort{}
	config := PortConfig{DTR: true, RTS: false, ReadTimeout: 2 * time.Second}
	if err := configureOpenedPort(port, config); err != nil {
		t.Fatalf("configureOpenedPort() error = %v", err)
	}
	if !port.dtr || port.rts || port.timeout != 2*time.Second {
		t.Fatalf("configured port = DTR %v RTS %v timeout %s", port.dtr, port.rts, port.timeout)
	}
}

func TestConfigureOpenedPortPropagatesErrors(t *testing.T) {
	sentinel := errors.New("configuration failed")
	tests := []struct {
		name string
		port *fakeConfigurablePort
	}{
		{name: "DTR", port: &fakeConfigurablePort{dtrError: sentinel}},
		{name: "RTS", port: &fakeConfigurablePort{rtsError: sentinel}},
		{name: "timeout", port: &fakeConfigurablePort{timeoutErr: sentinel}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := configureOpenedPort(tt.port, PortConfig{ReadTimeout: time.Second}); !errors.Is(err, sentinel) {
				t.Fatalf("configureOpenedPort() error = %v, want sentinel", err)
			}
		})
	}
}
