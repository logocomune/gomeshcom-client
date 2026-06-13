package main

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultAddress = "192.168.1.53:2323"
	password       = "pippo"
)

func main() {
	address := defaultAddress
	if len(os.Args) > 1 {
		address = os.Args[1]
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(os.Stderr, "connected to %s\n", address)

	reader := bufio.NewReader(conn)
	if err := authenticate(reader, conn); err != nil {
		fmt.Fprintf(os.Stderr, "auth failed: %v\n", err)
		os.Exit(1)
	}

	go func() {
		_, _ = io.Copy(os.Stdout, reader)
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := conn.Write([]byte(line + "\r\n")); err != nil {
			fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
			os.Exit(1)
		}
	}
}

func authenticate(reader *bufio.Reader, conn net.Conn) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	fmt.Print(line)

	nonceHex, ok := strings.CutPrefix(strings.TrimSpace(line), "NONCE: ")
	if !ok {
		return nil
	}

	nonce, err := hex.DecodeString(nonceHex)
	if err != nil {
		return err
	}

	mac := hmac.New(sha256.New, []byte(password))
	_, _ = mac.Write(nonce)
	response := hex.EncodeToString(mac.Sum(nil))

	if _, err := conn.Write([]byte(response + "\r\n")); err != nil {
		return err
	}

	result, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	fmt.Print(result)

	if strings.TrimSpace(result) != "OK" {
		return fmt.Errorf("%s", strings.TrimSpace(result))
	}

	return nil
}
