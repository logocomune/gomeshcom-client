// udpsend sends a single UDP datagram to a remote host.
//
// Usage:
//
//	udpsend -addr host:port -payload "your message"
//	udpsend -addr host:port -hex "48656c6c6f"
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	addr := flag.String("addr", "", "destination host:port (required)")
	payload := flag.String("payload", "", "payload to send as UTF-8 string")
	hexPayload := flag.String("hex", "", "payload to send as hex string (e.g. 48656c6c6f)")
	flag.Parse()

	if *addr == "" {
		fmt.Fprintln(os.Stderr, "error: -addr is required")
		flag.Usage()
		os.Exit(1)
	}
	if *payload == "" && *hexPayload == "" {
		fmt.Fprintln(os.Stderr, "error: -payload or -hex is required")
		flag.Usage()
		os.Exit(1)
	}
	if *payload != "" && *hexPayload != "" {
		fmt.Fprintln(os.Stderr, "error: -payload and -hex are mutually exclusive")
		os.Exit(1)
	}

	var data []byte
	if *hexPayload != "" {
		decoded, err := hex.DecodeString(strings.ReplaceAll(*hexPayload, " ", ""))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid hex: %v\n", err)
			os.Exit(1)
		}
		data = decoded
	} else {
		data = []byte(*payload)
	}

	conn, err := net.Dial("udp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: dial %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	n, err := conn.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: write: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("sent %d bytes to %s\n", n, *addr)
}
