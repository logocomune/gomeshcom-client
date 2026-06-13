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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/logocomune/gomeshcom-client/internal/apprestart"
	"github.com/logocomune/gomeshcom-client/internal/channelshow"
	"github.com/logocomune/gomeshcom-client/internal/chatlog"
	"github.com/logocomune/gomeshcom-client/internal/chatstatus"
	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/dmstats"
	"github.com/logocomune/gomeshcom-client/internal/events"
	"github.com/logocomune/gomeshcom-client/internal/httpapi"
	"github.com/logocomune/gomeshcom-client/internal/legacymigrate"
	"github.com/logocomune/gomeshcom-client/internal/logfmt"
	"github.com/logocomune/gomeshcom-client/internal/positions"
	"github.com/logocomune/gomeshcom-client/internal/receivelog"
	"github.com/logocomune/gomeshcom-client/internal/sendcache"
	"github.com/logocomune/gomeshcom-client/internal/station"
	"github.com/logocomune/gomeshcom-client/internal/stats"
	"github.com/logocomune/gomeshcom-client/internal/udpbridge"
	"github.com/logocomune/gomeshcom-client/internal/udpforward"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const startupBannerInnerWidth = 60

func main() {
	restart, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if restart {
		if apprestart.RunningInContainer() {
			// Inside a container: exit cleanly and let the container runtime
			// relaunch via its restart policy (e.g. docker-compose restart:
			// unless-stopped).
			os.Exit(0)
		}

		// Standalone: give the OS a moment to release the TCP/UDP ports and
		// file locks before the child tries to bind them.
		time.Sleep(500 * time.Millisecond)
		if err := apprestart.RestartSelf(); err != nil {
			fmt.Fprintln(os.Stderr, "restart failed:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func run() (bool, error) {
	cfg, envOverrides, info, err := config.Load(version)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) || errors.Is(err, conf.ErrVersionWanted) {
			fmt.Println(info)
			return false, nil
		}
		return false, fmt.Errorf("load config: %w", err)
	}
	configureLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// restartRequested is set to true when a graceful restart (not a clean
	// shutdown) was requested via POST /api/restart or SIGHUP.
	var restartRequested atomic.Bool

	// triggerRestart marks a restart intent and cancels the main context so
	// that the graceful HTTP+UDP shutdown sequence runs before main() decides
	// whether to exit or re-exec.
	triggerRestart := func() {
		restartRequested.Store(true)
		stop()
	}

	// Watch for SIGHUP as an additional restart trigger (useful for ops/scripts).
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		select {
		case <-hup:
			slog.Info("SIGHUP received, restarting")
			triggerRestart()
		case <-ctx.Done():
		}
	}()

	if err := ensureDataDirs(cfg.DataDir); err != nil {
		return false, err
	}

	// Runtime station identity: persisted value wins over GOMESHCOM_MY_CALL default.
	stationIdentity, err := station.New(station.DefaultPath(cfg.DataDir), cfg.MyCall)
	if err != nil {
		return false, fmt.Errorf("load station identity: %w", err)
	}
	go stationIdentity.Start(ctx)

	// DEPRECATED: one-time legacy data migration; remove after migration window closes.
	if err := legacymigrate.Run(cfg.DataDir, cfg.ChatLog.Path, stationIdentity.Current()); err != nil {
		return false, fmt.Errorf("legacy migration: %w", err)
	}

	bus := events.NewBus()
	positionStore := positions.New(positions.DefaultPath(cfg.DataDir))
	if err := positionStore.Load(); err != nil {
		return false, fmt.Errorf("load positions: %w", err)
	}
	go positionStore.Start(ctx)

	receiveLogger := receivelog.New(receivelog.Config{
		Enabled:       cfg.ReceiveLog.Enabled,
		Path:          cfg.ReceiveLog.Path,
		RetentionDays: cfg.ReceiveLog.RetentionDays,
	})
	chatLogger := chatlog.New(cfg.ChatLog.Path, stationIdentity)
	chatStatus, err := chatstatus.New(filepath.Join(cfg.ChatLog.Path, "msg_idx.json"))
	if err != nil {
		return false, fmt.Errorf("load chat status: %w", err)
	}
	go chatStatus.Start(ctx)

	channelShow, err := channelshow.New(channelshow.DefaultPath(cfg.DataDir))
	if err != nil {
		return false, fmt.Errorf("load channel show: %w", err)
	}
	go channelShow.Start(ctx)

	var statsStore *stats.Store
	if cfg.Stats.Enabled {
		statsStore = stats.New(stats.Config{
			Enabled:       true,
			Path:          cfg.Stats.Path,
			RetentionDays: cfg.Stats.RetentionDays,
		})
		if err := statsStore.Load(); err != nil {
			return false, fmt.Errorf("load stats: %w", err)
		}
		go statsStore.Start(ctx)
		collector := stats.NewCollector(statsStore, positionStore, stationIdentity)
		go collector.Run(ctx, bus)
	}

	dmStatsStore := dmstats.New(dmstats.DefaultPath(cfg.DataDir))
	if err := dmStatsStore.Load(); err != nil {
		return false, fmt.Errorf("load dm stats: %w", err)
	}
	go dmStatsStore.Start(ctx)

	var fwd *udpforward.Forwarder
	if cfg.Forward.Targets != "" {
		targets, err := config.ParseForwardTargets(cfg.Forward.Targets)
		if err != nil {
			return false, fmt.Errorf("udp forward targets: %w", err)
		}
		fwd, err = udpforward.New(targets)
		if err != nil {
			return false, fmt.Errorf("udp forwarder: %w", err)
		}
		defer fwd.Close()
	}

	bridge := udpbridge.NewBridge(cfg.UDPListenAddr, cfg.NodeAddr, bus, receiveLogger, chatLogger, positionStore, cfg.DemoMode, fwd, stationIdentity, chatStatus)
	go func() {
		if err := bridge.Listen(ctx); err != nil {
			slog.Error("udp bridge stopped", "error", err)
		}
	}()

	sc := sendcache.New(cfg.Send.DedupTTL)
	serverOpts := []httpapi.ServerOption{
		httpapi.WithChannelShow(channelShow),
		httpapi.WithStationIdentity(stationIdentity),
	}
	if statsStore != nil {
		serverOpts = append(serverOpts, httpapi.WithStats(statsStore))
	}
	serverOpts = append(serverOpts, httpapi.WithDMStats(dmStatsStore))
	serverOpts = append(serverOpts,
		httpapi.WithEnvOverrides(envOverrides),
		httpapi.WithTomlPath(config.DefaultTomlPath(cfg.DataDir)),
		httpapi.WithRestartFunc(triggerRestart),
		httpapi.WithShutdownFunc(stop),
	)
	apiServer := httpapi.NewServer(cfg, version, bus, positionStore, receiveLogger, chatLogger, bridge, sc, chatStatus, serverOpts...)
	defer apiServer.Close()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiServer.Handler(),
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

	printStartupBanner(cfg, stationIdentity.Current())

	slog.Info("gomeshcom listening", "http_addr", cfg.HTTPAddr, "udp_listen_addr", cfg.UDPListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return false, fmt.Errorf("serve http: %w", err)
	}

	return restartRequested.Load(), nil
}

func ensureDataDirs(dataDir string) error {
	for _, dir := range []string{"raw", "nodes", "chat", "stats", "configs"} {
		path := filepath.Join(dataDir, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create data directory %s: %w", path, err)
		}
	}

	return nil
}

func printStartupBanner(cfg config.Config, myCall string) {
	fmt.Print(startupBanner(cfg, myCall))
}

func startupBanner(cfg config.Config, myCall string) string {
	var b strings.Builder
	b.WriteString(bannerRule("="))
	b.WriteString(bannerText("GOMESHCOMD"))
	b.WriteString(bannerText("MeshCom UDP Link Terminal"))
	b.WriteString(bannerRule("-"))
	b.WriteString(bannerText("STATUS   READY"))
	b.WriteString(bannerText("VERSION  " + version))
	displayCall := myCall
	if displayCall == "" {
		displayCall = "(unset)"
	}
	b.WriteString(bannerText("MYCALL   " + displayCall))
	nodeDisplay := cfg.NodeAddr
	if nodeDisplay == "" {
		nodeDisplay = "(auto-detect from incoming UDP)"
	}
	b.WriteString(bannerText("NODE     " + nodeDisplay))
	b.WriteString(bannerText("HELP     gomeshcomd --help"))
	b.WriteString(bannerText("UDP RX   " + cfg.UDPListenAddr))
	b.WriteString(bannerText("WEB UI   " + webInterfaceURL(cfg.HTTPAddr)))
	if myCall == "" {
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

	logger := slog.New(logfmt.New(os.Stdout, level))
	slog.SetDefault(logger)
}
