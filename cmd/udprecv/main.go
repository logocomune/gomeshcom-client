// udprecv listens on a UDP address and prints each received datagram.
//
// Usage:
//
//	udprecv -addr :1799
//	udprecv -addr :1799 -hex
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":1799", "local address to listen on (host:port or :port)")
	asHex := flag.Bool("hex", false, "print payload as hex dump instead of UTF-8 string")
	bufSize := flag.Int("buf", 65535, "receive buffer size in bytes")
	flag.Parse()

	udpAddr, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolve %s: %v\n", *addr, err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(os.Stderr, "listening on %s (hex=%v)\n", conn.LocalAddr(), *asHex)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		conn.Close()
	}()

	buf := make([]byte, *bufSize)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			// conn closed via signal
			return
		}

		data := buf[:n]
		ts := time.Now().UTC().Format(time.RFC3339Nano)

		if *asHex {
			fmt.Printf("[%s] from=%s bytes=%d\n%s\n", ts, remote, n, hex.Dump(data))
		} else {
			fmt.Printf("[%s] from=%s bytes=%d payload=%q\n", ts, remote, n, data)
		}
	}
}
