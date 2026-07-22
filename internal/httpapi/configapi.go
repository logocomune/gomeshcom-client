package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/logocomune/gomeshcom-client/internal/config"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

// -------- Response DTO --------

// configFieldMeta wraps a single config value with metadata for the UI.
type configFieldMeta struct {
	Value           any  `json:"value"`
	EnvOverride     bool `json:"env_override"`
	RequiresRestart bool `json:"requires_restart"`
}

// serverInfo holds runtime-only fields that are not config but are useful alongside it.
type serverInfo struct {
	Version       string `json:"version"`
	StartedAt     string `json:"started_at"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// configResponse is the shape returned by GET /api/config.
// auth.password value is always masked — never the real value.
type configResponse struct {
	Server           serverInfo               `json:"server"`
	HTTPAddr         configFieldMeta          `json:"http_addr"`
	UDPListenAddr    configFieldMeta          `json:"udp_listen_addr"`
	NodeAddr         configFieldMeta          `json:"node_addr"`
	MyCall           configFieldMeta          `json:"my_call"`
	DataDir          configFieldMeta          `json:"data_dir"`
	MaxMessageLength configFieldMeta          `json:"max_message_length"`
	LogLevel         configFieldMeta          `json:"log_level"`
	ReceiveLog       configReceiveLogResponse `json:"receive_log"`
	Stats            configStatsResponse      `json:"stats"`
	ChatLog          configChatLogResponse    `json:"chat_log"`
	Send             configSendResponse       `json:"send"`
	Forward          configForwardResponse    `json:"forward"`
	Auth             configAuthResponse       `json:"auth"`
	RequestLog       configRequestLogResponse `json:"request_log"`
	Storage          configStorageResponse    `json:"storage"`
}

type configReceiveLogResponse struct {
	Enabled       configFieldMeta `json:"enabled"`
	Path          configFieldMeta `json:"path"`
	RetentionDays configFieldMeta `json:"retention_days"`
	ReplayWindow  configFieldMeta `json:"replay_window"`
}

type configStatsResponse struct {
	Enabled       configFieldMeta `json:"enabled"`
	Path          configFieldMeta `json:"path"`
	RetentionDays configFieldMeta `json:"retention_days"`
}

type configChatLogResponse struct {
	Path             configFieldMeta `json:"path"`
	HistoryWindow    configFieldMeta `json:"history_window"`
	MaxHistoryWindow configFieldMeta `json:"max_history_window"`
}

type configSendResponse struct {
	DedupTTL configFieldMeta `json:"dedup_ttl"`
}

type configForwardResponse struct {
	Targets configFieldMeta `json:"targets"`
}

type configAuthResponse struct {
	Username   configFieldMeta `json:"username"`
	Password   configFieldMeta `json:"password"` // value is always masked
	SessionTTL configFieldMeta `json:"session_ttl"`
	CookieName configFieldMeta `json:"cookie_name"`
}

type configRequestLogResponse struct {
	Enabled configFieldMeta `json:"enabled"`
}

type configStorageResponse struct {
	SQLitePath          configFieldMeta `json:"sqlite_path"`
	PurgeInterval       configFieldMeta `json:"purge_interval"`
	ReceiveLogRetention configFieldMeta `json:"receive_log_retention"`
	PublicChatRetention configFieldMeta `json:"public_chat_retention"`
	NodesRetention      configFieldMeta `json:"nodes_retention"`
	TelemetryRetention  configFieldMeta `json:"telemetry_retention"`
}

// -------- Update request DTO --------

// configUpdateRequest accepts a partial config update.
// Nil pointer = field not changing. All durations are strings (e.g. "40s").
type configUpdateRequest struct {
	HTTPAddr         *string `json:"http_addr,omitempty"`
	UDPListenAddr    *string `json:"udp_listen_addr,omitempty"`
	NodeAddr         *string `json:"node_addr,omitempty"`
	MyCall           *string `json:"my_call,omitempty"`
	MaxMessageLength *int    `json:"max_message_length,omitempty"`
	LogLevel         *string `json:"log_level,omitempty"`

	ReceiveLog *configUpdateReceiveLog `json:"receive_log,omitempty"`
	Stats      *configUpdateStats      `json:"stats,omitempty"`
	ChatLog    *configUpdateChatLog    `json:"chat_log,omitempty"`
	Send       *configUpdateSend       `json:"send,omitempty"`
	Forward    *configUpdateForward    `json:"forward,omitempty"`
	Auth       *configUpdateAuth       `json:"auth,omitempty"`
	RequestLog *configUpdateRequestLog `json:"request_log,omitempty"`
	Storage    *configUpdateStorage    `json:"storage,omitempty"`
}

type configUpdateReceiveLog struct {
	Enabled       *bool   `json:"enabled,omitempty"`
	Path          *string `json:"path,omitempty"`
	RetentionDays *int    `json:"retention_days,omitempty"`
	ReplayWindow  *string `json:"replay_window,omitempty"`
}

type configUpdateStats struct {
	Enabled       *bool   `json:"enabled,omitempty"`
	Path          *string `json:"path,omitempty"`
	RetentionDays *int    `json:"retention_days,omitempty"`
}

type configUpdateChatLog struct {
	Path             *string `json:"path,omitempty"`
	HistoryWindow    *string `json:"history_window,omitempty"`
	MaxHistoryWindow *string `json:"max_history_window,omitempty"`
}

type configUpdateSend struct {
	DedupTTL *string `json:"dedup_ttl,omitempty"`
}

type configUpdateForward struct {
	Targets *string `json:"targets,omitempty"`
}

type configUpdateAuth struct {
	Username   *string `json:"username,omitempty"`
	Password   *string `json:"password,omitempty"`
	SessionTTL *string `json:"session_ttl,omitempty"`
	CookieName *string `json:"cookie_name,omitempty"`
}

type configUpdateRequestLog struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type configUpdateStorage struct {
	SQLitePath          *string `json:"sqlite_path,omitempty"`
	PurgeInterval       *string `json:"purge_interval,omitempty"`
	ReceiveLogRetention *string `json:"receive_log_retention,omitempty"`
	PublicChatRetention *string `json:"public_chat_retention,omitempty"`
	NodesRetention      *string `json:"nodes_retention,omitempty"`
	TelemetryRetention  *string `json:"telemetry_retention,omitempty"`
}

// passwordMask is the sentinel value returned for the password field in GET responses.
const passwordMask = "****"

// -------- Helpers --------

func field(value any, envKey string, env config.EnvOverrides, restart bool) configFieldMeta {
	return configFieldMeta{
		Value:           value,
		EnvOverride:     env[envKey],
		RequiresRestart: restart,
	}
}

func durationField(d time.Duration, envKey string, env config.EnvOverrides, restart bool) configFieldMeta {
	return field(d.String(), envKey, env, restart)
}

func buildConfigResponse(cfg config.Config, env config.EnvOverrides, version string, startedAt time.Time) configResponse {
	// auth.password: show mask if set, empty otherwise — never the real value.
	passwordDisplay := ""
	if cfg.Auth.Password != "" {
		passwordDisplay = passwordMask
	}

	now := time.Now().UTC()
	uptimeSecs := int64(0)
	if !startedAt.IsZero() {
		uptimeSecs = int64(now.Sub(startedAt).Seconds())
	}

	return configResponse{
		Server: serverInfo{
			Version:       version,
			StartedAt:     startedAt.Format(time.RFC3339),
			UptimeSeconds: uptimeSecs,
		},
		HTTPAddr:         field(cfg.HTTPAddr, "HTTP_ADDR", env, true),
		UDPListenAddr:    field(cfg.UDPListenAddr, "UDP_LISTEN_ADDR", env, true),
		NodeAddr:         field(cfg.NodeAddr, "NODE_ADDR", env, true),
		MyCall:           field(cfg.MyCall, "MY_CALL", env, false),
		DataDir:          field(cfg.DataDir, "DATA_DIR", env, true),
		MaxMessageLength: field(cfg.MaxMessageLength, "MAX_MESSAGE_LENGTH", env, true),
		LogLevel:         field(cfg.LogLevel, "LOG_LEVEL", env, false),
		ReceiveLog: configReceiveLogResponse{
			Enabled:       field(cfg.ReceiveLog.Enabled, "RECEIVE_LOG_ENABLED", env, true),
			Path:          field(cfg.ReceiveLog.Path, "RECEIVE_LOG_PATH", env, true),
			RetentionDays: field(cfg.ReceiveLog.RetentionDays, "RECEIVE_LOG_RETENTION_DAYS", env, true),
			ReplayWindow:  durationField(cfg.ReceiveLog.ReplayWindow, "RECEIVE_LOG_REPLAY_WINDOW", env, true),
		},
		Stats: configStatsResponse{
			Enabled:       field(cfg.Stats.Enabled, "STATS_ENABLED", env, true),
			Path:          field(cfg.Stats.Path, "STATS_PATH", env, true),
			RetentionDays: field(cfg.Stats.RetentionDays, "STATS_RETENTION_DAYS", env, true),
		},
		ChatLog: configChatLogResponse{
			Path:             field(cfg.ChatLog.Path, "CHAT_LOG_PATH", env, true),
			HistoryWindow:    durationField(cfg.ChatLog.HistoryWindow, "CHAT_LOG_HISTORY_WINDOW", env, false),
			MaxHistoryWindow: durationField(cfg.ChatLog.MaxHistoryWindow, "CHAT_LOG_MAX_HISTORY_WINDOW", env, false),
		},
		Send: configSendResponse{
			DedupTTL: durationField(cfg.Send.DedupTTL, "SEND_DEDUP_TTL", env, false),
		},
		Forward: configForwardResponse{
			Targets: field(cfg.Forward.Targets, "FORWARD_TARGETS", env, true),
		},
		Auth: configAuthResponse{
			Username:   field(cfg.Auth.Username, "AUTH_USERNAME", env, true),
			Password:   field(passwordDisplay, "AUTH_PASSWORD", env, true),
			SessionTTL: durationField(cfg.Auth.SessionTTL, "AUTH_SESSION_TTL", env, true),
			CookieName: field(cfg.Auth.CookieName, "AUTH_COOKIE_NAME", env, true),
		},
		RequestLog: configRequestLogResponse{
			Enabled: field(cfg.RequestLog.Enabled, "REQUEST_LOG_ENABLED", env, false),
		},
		Storage: configStorageResponse{
			SQLitePath:          field(cfg.Storage.SQLitePath, "STORAGE_SQLITE_PATH", env, true),
			PurgeInterval:       durationField(cfg.Storage.PurgeInterval, "STORAGE_PURGE_INTERVAL", env, true),
			ReceiveLogRetention: durationField(cfg.Storage.ReceiveLogRetention, "STORAGE_RECEIVE_LOG_RETENTION", env, true),
			PublicChatRetention: durationField(cfg.Storage.PublicChatRetention, "STORAGE_PUBLIC_CHAT_RETENTION", env, true),
			NodesRetention:      durationField(cfg.Storage.NodesRetention, "STORAGE_NODES_RETENTION", env, true),
			TelemetryRetention:  durationField(cfg.Storage.TelemetryRetention, "STORAGE_TELEMETRY_RETENTION", env, true),
		},
	}
}

// -------- Handlers --------

// getConfig handles GET /api/config.
func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.DemoMode {
		writeError(w, http.StatusForbidden, "config API disabled in demo mode")
		return
	}
	writeJSON(w, http.StatusOK, buildConfigResponse(s.cfg, s.envOverrides, s.version, s.startedAt))
}

// updateConfig handles PUT /api/config.
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.DemoMode {
		writeError(w, http.StatusForbidden, "config API disabled in demo mode")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<14) // 16 KB

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	var req configUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Build candidate config by merging the request onto the current effective config.
	candidate := s.cfg

	// Reject attempts to override env-managed fields.
	if err := checkEnvLocked(req, s.envOverrides); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	requiresRestart := false
	myCallChanged := false

	// Apply top-level fields.
	if req.HTTPAddr != nil {
		candidate.HTTPAddr = *req.HTTPAddr
		requiresRestart = true
	}
	if req.UDPListenAddr != nil {
		candidate.UDPListenAddr = *req.UDPListenAddr
		requiresRestart = true
	}
	if req.NodeAddr != nil {
		candidate.NodeAddr = *req.NodeAddr
		requiresRestart = true
	}
	if req.MyCall != nil {
		candidate.MyCall = *req.MyCall
		myCallChanged = true
	}
	if req.MaxMessageLength != nil {
		candidate.MaxMessageLength = *req.MaxMessageLength
		requiresRestart = true
	}
	if req.LogLevel != nil {
		candidate.LogLevel = *req.LogLevel
	}

	// Sections.
	if req.ReceiveLog != nil {
		rl := req.ReceiveLog
		if rl.Enabled != nil {
			candidate.ReceiveLog.Enabled = *rl.Enabled
			requiresRestart = true
		}
		if rl.Path != nil {
			candidate.ReceiveLog.Path = *rl.Path
			requiresRestart = true
		}
		if rl.RetentionDays != nil {
			candidate.ReceiveLog.RetentionDays = *rl.RetentionDays
			requiresRestart = true
		}
		if rl.ReplayWindow != nil {
			d, err := time.ParseDuration(*rl.ReplayWindow)
			if err != nil {
				writeError(w, http.StatusBadRequest, "receive_log.replay_window: "+err.Error())
				return
			}
			candidate.ReceiveLog.ReplayWindow = d
			requiresRestart = true
		}
	}
	if req.Stats != nil {
		s2 := req.Stats
		if s2.Enabled != nil {
			candidate.Stats.Enabled = *s2.Enabled
			requiresRestart = true
		}
		if s2.Path != nil {
			candidate.Stats.Path = *s2.Path
			requiresRestart = true
		}
		if s2.RetentionDays != nil {
			candidate.Stats.RetentionDays = *s2.RetentionDays
			requiresRestart = true
		}
	}
	if req.ChatLog != nil {
		cl := req.ChatLog
		if cl.Path != nil {
			candidate.ChatLog.Path = *cl.Path
			requiresRestart = true
		}
		if cl.HistoryWindow != nil {
			d, err := time.ParseDuration(*cl.HistoryWindow)
			if err != nil {
				writeError(w, http.StatusBadRequest, "chat_log.history_window: "+err.Error())
				return
			}
			candidate.ChatLog.HistoryWindow = d
		}
		if cl.MaxHistoryWindow != nil {
			d, err := time.ParseDuration(*cl.MaxHistoryWindow)
			if err != nil {
				writeError(w, http.StatusBadRequest, "chat_log.max_history_window: "+err.Error())
				return
			}
			candidate.ChatLog.MaxHistoryWindow = d
		}
	}
	if req.Send != nil {
		snd := req.Send
		if snd.DedupTTL != nil {
			d, err := time.ParseDuration(*snd.DedupTTL)
			if err != nil {
				writeError(w, http.StatusBadRequest, "send.dedup_ttl: "+err.Error())
				return
			}
			candidate.Send.DedupTTL = d
		}
	}
	if req.Forward != nil {
		if req.Forward.Targets != nil {
			candidate.Forward.Targets = *req.Forward.Targets
			requiresRestart = true
		}
	}
	if req.Auth != nil {
		a := req.Auth
		if a.Username != nil {
			candidate.Auth.Username = *a.Username
			requiresRestart = true
		}
		// Password: only apply when non-empty and not the mask sentinel.
		if a.Password != nil && *a.Password != "" && *a.Password != passwordMask {
			candidate.Auth.Password = *a.Password
			requiresRestart = true
		}
		if a.SessionTTL != nil {
			d, err := time.ParseDuration(*a.SessionTTL)
			if err != nil {
				writeError(w, http.StatusBadRequest, "auth.session_ttl: "+err.Error())
				return
			}
			candidate.Auth.SessionTTL = d
			requiresRestart = true
		}
		if a.CookieName != nil {
			candidate.Auth.CookieName = *a.CookieName
			requiresRestart = true
		}
	}
	if req.RequestLog != nil {
		if req.RequestLog.Enabled != nil {
			candidate.RequestLog.Enabled = *req.RequestLog.Enabled
		}
	}
	if req.Storage != nil {
		st := req.Storage
		if st.SQLitePath != nil {
			candidate.Storage.SQLitePath = *req.Storage.SQLitePath
			requiresRestart = true
		}
		if st.PurgeInterval != nil {
			d, err := config.ParseDuration(*st.PurgeInterval)
			if err != nil {
				writeError(w, http.StatusBadRequest, "storage.purge_interval: "+err.Error())
				return
			}
			candidate.Storage.PurgeInterval = d
			requiresRestart = true
		}
		if st.ReceiveLogRetention != nil {
			d, err := config.ParseDuration(*st.ReceiveLogRetention)
			if err != nil {
				writeError(w, http.StatusBadRequest, "storage.receive_log_retention: "+err.Error())
				return
			}
			candidate.Storage.ReceiveLogRetention = d
			requiresRestart = true
		}
		if st.PublicChatRetention != nil {
			d, err := config.ParseDuration(*st.PublicChatRetention)
			if err != nil {
				writeError(w, http.StatusBadRequest, "storage.public_chat_retention: "+err.Error())
				return
			}
			candidate.Storage.PublicChatRetention = d
			requiresRestart = true
		}
		if st.NodesRetention != nil {
			d, err := config.ParseDuration(*st.NodesRetention)
			if err != nil {
				writeError(w, http.StatusBadRequest, "storage.nodes_retention: "+err.Error())
				return
			}
			candidate.Storage.NodesRetention = d
			requiresRestart = true
		}
		if st.TelemetryRetention != nil {
			d, err := config.ParseDuration(*st.TelemetryRetention)
			if err != nil {
				writeError(w, http.StatusBadRequest, "storage.telemetry_retention: "+err.Error())
				return
			}
			candidate.Storage.TelemetryRetention = d
			requiresRestart = true
		}
	}

	// Normalize and validate before persisting.
	if err := config.Validate(candidate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Persist to TOML atomically.
	if s.tomlPath != "" {
		if err := config.WriteToml(s.tomlPath, candidate); err != nil {
			slog.Error("config save failed", "error", err)
			writeError(w, http.StatusInternalServerError, "persist config failed")
			return
		}
	}

	// Apply live-applyable fields.
	if myCallChanged && s.identity != nil {
		accepted, err := s.identity.Update(candidate.MyCall)
		if err != nil {
			// Validation passed above, but identity.Update may normalize differently.
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		candidate.MyCall = accepted

		if saveErr := s.identity.SaveIfDirty(); saveErr != nil {
			slog.Error("station identity save failed after config update", "error", saveErr)
		}

		if s.bus != nil {
			fwdTargets, _ := config.ParseForwardTargets(candidate.Forward.Targets)
			s.bus.Publish(events.Event{
				Type: "station.identity",
				Data: stationIdentityEvent{
					Callsign:           accepted,
					Version:            s.version,
					TxDisabled:         candidate.DemoMode,
					ForwardTargetCount: len(fwdTargets),
				},
			})
		}
	}

	// Update the server's in-memory config.
	s.cfg = candidate

	type updateResponse struct {
		Config          configResponse `json:"config"`
		RequiresRestart bool           `json:"requires_restart"`
	}
	writeJSON(w, http.StatusOK, updateResponse{
		Config:          buildConfigResponse(s.cfg, s.envOverrides, s.version, s.startedAt),
		RequiresRestart: requiresRestart,
	})
}

// checkEnvLocked returns an error if the update attempts to change any field
// that is currently managed by an environment variable.
func checkEnvLocked(req configUpdateRequest, env config.EnvOverrides) error {
	type check struct {
		present bool
		suffix  string
	}

	checks := []check{
		{req.HTTPAddr != nil, "HTTP_ADDR"},
		{req.UDPListenAddr != nil, "UDP_LISTEN_ADDR"},
		{req.NodeAddr != nil, "NODE_ADDR"},
		{req.MyCall != nil, "MY_CALL"},
		{req.MaxMessageLength != nil, "MAX_MESSAGE_LENGTH"},
		{req.LogLevel != nil, "LOG_LEVEL"},
	}
	if req.ReceiveLog != nil {
		rl := req.ReceiveLog
		checks = append(checks,
			check{rl.Enabled != nil, "RECEIVE_LOG_ENABLED"},
			check{rl.Path != nil, "RECEIVE_LOG_PATH"},
			check{rl.RetentionDays != nil, "RECEIVE_LOG_RETENTION_DAYS"},
			check{rl.ReplayWindow != nil, "RECEIVE_LOG_REPLAY_WINDOW"},
		)
	}
	if req.Stats != nil {
		s2 := req.Stats
		checks = append(checks,
			check{s2.Enabled != nil, "STATS_ENABLED"},
			check{s2.Path != nil, "STATS_PATH"},
			check{s2.RetentionDays != nil, "STATS_RETENTION_DAYS"},
		)
	}
	if req.ChatLog != nil {
		cl := req.ChatLog
		checks = append(checks,
			check{cl.Path != nil, "CHAT_LOG_PATH"},
			check{cl.HistoryWindow != nil, "CHAT_LOG_HISTORY_WINDOW"},
			check{cl.MaxHistoryWindow != nil, "CHAT_LOG_MAX_HISTORY_WINDOW"},
		)
	}
	if req.Send != nil {
		snd := req.Send
		checks = append(checks,
			check{snd.DedupTTL != nil, "SEND_DEDUP_TTL"},
		)
	}
	if req.Forward != nil && req.Forward.Targets != nil {
		checks = append(checks, check{true, "FORWARD_TARGETS"})
	}
	if req.Auth != nil {
		a := req.Auth
		checks = append(checks,
			check{a.Username != nil, "AUTH_USERNAME"},
			check{a.Password != nil && *a.Password != "" && *a.Password != passwordMask, "AUTH_PASSWORD"},
			check{a.SessionTTL != nil, "AUTH_SESSION_TTL"},
			check{a.CookieName != nil, "AUTH_COOKIE_NAME"},
		)
	}
	if req.RequestLog != nil && req.RequestLog.Enabled != nil {
		checks = append(checks, check{true, "REQUEST_LOG_ENABLED"})
	}
	if req.Storage != nil {
		st := req.Storage
		checks = append(checks,
			check{st.SQLitePath != nil, "STORAGE_SQLITE_PATH"},
			check{st.PurgeInterval != nil, "STORAGE_PURGE_INTERVAL"},
			check{st.ReceiveLogRetention != nil, "STORAGE_RECEIVE_LOG_RETENTION"},
			check{st.PublicChatRetention != nil, "STORAGE_PUBLIC_CHAT_RETENTION"},
			check{st.NodesRetention != nil, "STORAGE_NODES_RETENTION"},
			check{st.TelemetryRetention != nil, "STORAGE_TELEMETRY_RETENTION"},
		)
	}

	for _, c := range checks {
		if c.present && env[c.suffix] {
			return fmt.Errorf("field %s is managed by environment variable GOMESHCOM_%s and cannot be changed via API", c.suffix, c.suffix)
		}
	}
	return nil
}
