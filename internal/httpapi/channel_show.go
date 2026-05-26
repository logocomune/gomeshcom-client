package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/logocomune/gomeshcom-client/internal/channelshow"
	"github.com/logocomune/gomeshcom-client/internal/events"
)

func (s *Server) getChannelShow(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.channelShowSnapshot())
}

func (s *Server) updateChannelShow(w http.ResponseWriter, r *http.Request) {
	if s.channelShow == nil {
		writeError(w, http.StatusServiceUnavailable, "channel show store not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16) // 64 KB
	request, err := decodeChannelShowConfig(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	config, err := s.channelShow.Update(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.bus.Publish(events.Event{Type: "channelshow.snapshot", Data: config})
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) channelShowSnapshot() channelshow.Config {
	if s.channelShow == nil {
		return channelshow.DefaultConfig()
	}
	return s.channelShow.Snapshot()
}

func decodeChannelShowConfig(reader io.Reader) (channelshow.Config, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return channelshow.Config{}, fmt.Errorf("read json: %w", err)
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return channelshow.Config{}, fmt.Errorf("invalid json")
	}

	if trimmed[0] == '[' {
		var channels []string
		if err := json.Unmarshal(trimmed, &channels); err != nil {
			return channelshow.Config{}, fmt.Errorf("invalid json")
		}
		return channelshow.Config{Mode: channelshow.ModeAllowlist, Channels: channels}, nil
	}

	var config channelshow.Config
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return channelshow.Config{}, fmt.Errorf("invalid json")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return channelshow.Config{}, fmt.Errorf("invalid json")
	}
	return config, nil
}
