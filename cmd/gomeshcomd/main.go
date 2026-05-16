package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/logocomune/gomeshcom-udp/internal/chatlog"
	"github.com/logocomune/gomeshcom-udp/internal/config"
	"github.com/logocomune/gomeshcom-udp/internal/events"
	"github.com/logocomune/gomeshcom-udp/internal/httpapi"
	"github.com/logocomune/gomeshcom-udp/internal/positions"
	"github.com/logocomune/gomeshcom-udp/internal/receivelog"
	"github.com/logocomune/gomeshcom-udp/internal/sendcache"
	"github.com/logocomune/gomeshcom-udp/internal/udpbridge"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const startupBannerInnerWidth = 60

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, info, err := config.Load(version)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) || errors.Is(err, conf.ErrVersionWanted) {
			fmt.Println(info)
			return nil
		}
		return fmt.Errorf("load config: %w", err)
	}
	configureLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := ensureDataDirs(cfg.DataDir); err != nil {
		return err
	}

	bus := events.NewBus()
	positionStore := positions.New(positions.DefaultPath(cfg.DataDir))
	if err := positionStore.Load(); err != nil {
		return fmt.Errorf("load positions: %w", err)
	}
	go positionStore.Start(ctx)

	receiveLogger := receivelog.New(receivelog.Config{
		Enabled:       cfg.ReceiveLog.Enabled,
		Path:          cfg.ReceiveLog.Path,
		RetentionDays: cfg.ReceiveLog.RetentionDays,
	})
	chatLogger := chatlog.New(cfg.ChatLog.Path, cfg.MyCall)
	bridge := udpbridge.NewBridge(cfg.UDPListenAddr, cfg.NodeAddr, bus, receiveLogger, chatLogger, positionStore)
	go func() {
		if err := bridge.Listen(ctx); err != nil {
			slog.Error("udp bridge stopped", "error", err)
		}
	}()

	sc := sendcache.New(cfg.Send.DedupTTL)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewServer(cfg, version, bus, positionStore, receiveLogger, chatLogger, bridge, sc).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("http shutdown failed", "error", err)
		}
	}()

	printStartupBanner(cfg)

	slog.Info("gomeshcom listening", "http_addr", cfg.HTTPAddr, "udp_listen_addr", cfg.UDPListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}

func ensureDataDirs(dataDir string) error {
	for _, dir := range []string{"raw", "nodes", "chat"} {
		path := filepath.Join(dataDir, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create data directory %s: %w", path, err)
		}
	}

	return nil
}

func printStartupBanner(cfg config.Config) {
	fmt.Print(startupBanner(cfg))
}

func startupBanner(cfg config.Config) string {
	var b strings.Builder
	b.WriteString(bannerRule("="))
	b.WriteString(bannerText("GOMESHCOMD"))
	b.WriteString(bannerText("MeshCom UDP Link Terminal"))
	b.WriteString(bannerRule("-"))
	b.WriteString(bannerText("STATUS   READY"))
	b.WriteString(bannerText("VERSION  " + version))
	myCall := cfg.MyCall
	if myCall == "" {
		myCall = "(unset)"
	}
	b.WriteString(bannerText("MYCALL   " + myCall))
	nodeDisplay := cfg.NodeAddr
	if nodeDisplay == "" {
		nodeDisplay = "(auto-detect from incoming UDP)"
	}
	b.WriteString(bannerText("NODE     " + nodeDisplay))
	b.WriteString(bannerText("HELP     gomeshcomd --help"))
	b.WriteString(bannerText("UDP RX   " + cfg.UDPListenAddr))
	b.WriteString(bannerText("WEB UI   " + webInterfaceURL(cfg.HTTPAddr)))
	if cfg.MyCall == "" {
		b.WriteString(bannerText("DMs      hidden until MyCall set"))
		b.WriteString(bannerText("MSG      node addr required"))
	}
	b.WriteString(bannerRule("="))
	return b.String()
}

func bannerRule(char string) string {
	return "+" + strings.Repeat(char, startupBannerInnerWidth) + "+\n"
}

func bannerText(text string) string {
	return fmt.Sprintf("| %-*s |\n", startupBannerInnerWidth-2, text)
}

func webInterfaceURL(httpAddr string) string {
	host, port, err := net.SplitHostPort(httpAddr)
	if err != nil {
		return "http://" + httpAddr
	}

	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	return "http://" + net.JoinHostPort(host, port)
}

func configureLogger(levelName string) {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	level, ok := levels[levelName]
	if !ok {
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
}
