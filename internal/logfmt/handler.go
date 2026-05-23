package logfmt

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

const timeFormat = "2006-01-02 15:04:05"

// Handler is a human-friendly slog.Handler that writes columnar log lines:
//
//	2006-01-02 15:04:05  LEVEL  message  key=value key2=value2
type Handler struct {
	w      io.Writer
	level  slog.Leveler
	mu     *sync.Mutex
	prefix string
	attrs  []slog.Attr
}

// New returns a Handler that writes to w, filtering records below the given level.
func New(w io.Writer, level slog.Leveler) *Handler {
	return &Handler{w: w, level: level, mu: &sync.Mutex{}}
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder
	sb.WriteString(r.Time.Format(timeFormat))
	sb.WriteString("  ")
	sb.WriteString(levelLabel(r.Level))
	sb.WriteString("  ")
	sb.WriteString(r.Message)
	for _, a := range h.attrs {
		appendAttr(&sb, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		a.Key = h.prefix + a.Key
		appendAttr(&sb, a)
		return true
	})
	sb.WriteByte('\n')

	h.mu.Lock()
	_, err := fmt.Fprint(h.w, sb.String())
	h.mu.Unlock()
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(cloned, h.attrs)
	for _, a := range attrs {
		a.Key = h.prefix + a.Key
		cloned = append(cloned, a)
	}
	return &Handler{w: h.w, level: h.level, mu: h.mu, prefix: h.prefix, attrs: cloned}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{w: h.w, level: h.level, mu: h.mu, prefix: h.prefix + name + ".", attrs: h.attrs}
}

func levelLabel(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO "
	case slog.LevelWarn:
		return "WARN "
	case slog.LevelError:
		return "ERROR"
	default:
		s := l.String()
		if len(s) < 5 {
			return s + strings.Repeat(" ", 5-len(s))
		}
		return s[:5]
	}
}

func appendAttr(sb *strings.Builder, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			if a.Key != "" {
				appendAttr(sb, slog.Attr{Key: a.Key + "." + ga.Key, Value: ga.Value})
			} else {
				appendAttr(sb, ga)
			}
		}
		return
	}
	if a.Key == "" {
		return
	}
	sb.WriteString("  ")
	sb.WriteString(a.Key)
	sb.WriteByte('=')
	appendValue(sb, a.Value)
}

func appendValue(sb *strings.Builder, v slog.Value) {
	var s string
	if v.Kind() == slog.KindString {
		s = v.String()
	} else {
		s = fmt.Sprintf("%v", v.Any())
	}
	if needsQuoting(s) {
		sb.WriteString(strconv.Quote(s))
	} else {
		sb.WriteString(s)
	}
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c == ' ' || c == '=' || c == '"' || c == '\\' || c == '\n' || c == '\r' || c == '\t' {
			return true
		}
	}
	return false
}
