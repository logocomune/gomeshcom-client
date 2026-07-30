package serialbridge

import (
	"fmt"
	"time"

	goserial "go.bug.st/serial"
)

type Port interface {
	Read(buffer []byte) (int, error)
	Write(data []byte) (int, error)
	Close() error
}

type PortConfig struct {
	Device      string
	Baud        int
	DataBits    int
	Parity      string
	StopBits    int
	DTR         bool
	RTS         bool
	ReadTimeout time.Duration
}

type PortOpener interface {
	Open(config PortConfig) (Port, error)
}

type configurablePort interface {
	Port
	SetDTR(value bool) error
	SetRTS(value bool) error
	SetReadTimeout(timeout time.Duration) error
}

type LibraryPortOpener struct{}

func (LibraryPortOpener) Open(config PortConfig) (Port, error) {
	mode, err := libraryMode(config)
	if err != nil {
		return nil, err
	}
	port, err := goserial.Open(config.Device, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial device %s: %w", config.Device, err)
	}
	if err := configureOpenedPort(port, config); err != nil {
		_ = port.Close()
		return nil, err
	}
	return port, nil
}

func libraryMode(config PortConfig) (*goserial.Mode, error) {
	parity, err := libraryParity(config.Parity)
	if err != nil {
		return nil, err
	}
	stopBits, err := libraryStopBits(config.StopBits)
	if err != nil {
		return nil, err
	}
	return &goserial.Mode{
		BaudRate: config.Baud,
		DataBits: config.DataBits,
		Parity:   parity,
		StopBits: stopBits,
		InitialStatusBits: &goserial.ModemOutputBits{
			DTR: config.DTR,
			RTS: config.RTS,
		},
	}, nil
}

func libraryParity(parity string) (goserial.Parity, error) {
	switch parity {
	case "none":
		return goserial.NoParity, nil
	case "odd":
		return goserial.OddParity, nil
	case "even":
		return goserial.EvenParity, nil
	case "mark":
		return goserial.MarkParity, nil
	case "space":
		return goserial.SpaceParity, nil
	default:
		return goserial.NoParity, fmt.Errorf("unsupported serial parity %q", parity)
	}
}

func libraryStopBits(stopBits int) (goserial.StopBits, error) {
	switch stopBits {
	case 1:
		return goserial.OneStopBit, nil
	case 2:
		return goserial.TwoStopBits, nil
	default:
		return goserial.OneStopBit, fmt.Errorf("unsupported serial stop bits %d", stopBits)
	}
}

func configureOpenedPort(port configurablePort, config PortConfig) error {
	if err := port.SetDTR(config.DTR); err != nil {
		return fmt.Errorf("set serial DTR: %w", err)
	}
	if err := port.SetRTS(config.RTS); err != nil {
		return fmt.Errorf("set serial RTS: %w", err)
	}
	if err := port.SetReadTimeout(config.ReadTimeout); err != nil {
		return fmt.Errorf("set serial read timeout: %w", err)
	}
	return nil
}
