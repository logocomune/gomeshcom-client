package httpapi

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// -------- acceptsGzip --------

func TestAcceptsGzip(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"gzip, deflate", true},
		{"gzip;q=1.0", true},
		{"gzip;q=0.5", true},
		{"gzip;q=0", false},
		{"gzip;q=0.0", false},
		{"*", true},
		{"*, gzip;q=0", false},
		{"deflate, gzip;q=0", false},
		{"deflate", false},
		{"identity", false},
		{"", false},
		{"GZIP", true},
		{"  gzip  ", true},
		{"gzip ; q=1.0", true},
		{"xgzip", false},          // no substring match
		{"gzipx", false},          // exact coding only
		{"gzip;q=bad", false},     // malformed quality → 0
		{"gzip;q=-1", false},      // negative → 0
		{"*, gzip;q=0.0", false},  // explicit zero overrides wildcard
		{"deflate, *;q=1", true},  // wildcard covers gzip
		{"deflate, *;q=0", false}, // wildcard disabled
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := acceptsGzip(tt.header); got != tt.want {
				t.Errorf("acceptsGzip(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

// -------- isCompressible --------

func TestIsCompressible(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/html; charset=utf-8", true},
		{"text/plain", true},
		{"text/css", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/javascript", true},
		{"application/x-javascript", true},
		{"application/xml", true},
		{"application/xhtml+xml", true},
		{"image/svg+xml", true},
		{"image/png", false},
		{"image/jpeg", false},
		{"image/webp", false},
		{"image/gif", false},
		{"video/mp4", false},
		{"audio/mpeg", false},
		{"application/zip", false},
		{"application/gzip", false},
		{"application/octet-stream", false},
		{"font/woff2", false},
		{"", false},
		{"TEXT/HTML", true}, // case-insensitive
		{"APPLICATION/JSON", true},
	}
	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			if got := isCompressible(tt.ct); got != tt.want {
				t.Errorf("isCompressible(%q) = %v, want %v", tt.ct, got, tt.want)
			}
		})
	}
}

// -------- addVary --------

func TestAddVary(t *testing.T) {
	h := http.Header{}
	addVary(h, "Accept-Encoding")
	if got := h.Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want %q", got, "Accept-Encoding")
	}
	// Idempotent.
	addVary(h, "Accept-Encoding")
	if got := h.Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary after duplicate add = %q, want %q", got, "Accept-Encoding")
	}
	// Appends when different value present.
	h2 := http.Header{}
	h2.Set("Vary", "Origin")
	addVary(h2, "Accept-Encoding")
	if got := h2.Get("Vary"); got != "Origin, Accept-Encoding" {
		t.Errorf("Vary with existing = %q, want %q", got, "Origin, Accept-Encoding")
	}
}

// -------- compressionMiddleware --------

const minSize = 64

var testOpts = compressionOptions{MinimumSize: minSize, GzipLevel: gzip.BestSpeed}

func makeHandler(body string, ct string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = io.WriteString(w, body)
	})
}

func largeBody(n int) string {
	return strings.Repeat("hello world abcdefghij ", n)
}

func decompressGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll gzip: %v", err)
	}
	return out
}

func TestCompressionMiddleware_CompressesEligibleLargeResponse(t *testing.T) {
	body := largeBody(10) // well above 64 bytes
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected Content-Encoding: gzip")
	}
	if rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding", rec.Header().Get("Vary"))
	}
	got := decompressGzip(t, rec.Body.Bytes())
	if string(got) != body {
		t.Errorf("decompressed body mismatch: got %q, want %q", got, body)
	}
}

func TestCompressionMiddleware_NoGzipHeader(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	// No Accept-Encoding header.
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding")
	}
	if rec.Body.String() != body {
		t.Error("body mismatch for non-gzip client")
	}
}

func TestCompressionMiddleware_GzipQZero(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip;q=0")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding when gzip;q=0")
	}
}

func TestCompressionMiddleware_SmallResponseNotCompressed(t *testing.T) {
	body := "small"
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for small response")
	}
	if rec.Body.String() != body {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}
}

func TestCompressionMiddleware_ExistingContentEncodingPreserved(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "identity")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "identity" {
		t.Errorf("Content-Encoding = %q, want identity", rec.Header().Get("Content-Encoding"))
	}
}

func TestCompressionMiddleware_NoContentNotCompressed(t *testing.T) {
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for 204")
	}
}

func TestCompressionMiddleware_NotModifiedNotCompressed(t *testing.T) {
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for 304")
	}
}

func TestCompressionMiddleware_NonCompressibleContentType(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, makeHandler(body, "image/png"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/img.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for image/png")
	}
	if rec.Body.String() != body {
		t.Error("body mismatch for non-compressible content")
	}
}

func TestCompressionMiddleware_RangeRequestBypassed(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-100")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for Range request")
	}
}

func TestCompressionMiddleware_SSEEndpointBypassed(t *testing.T) {
	flushed := false
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: hello\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for SSE endpoint")
	}
	if rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding on SSE", rec.Header().Get("Vary"))
	}
	if !flushed {
		t.Error("expected SSE Flush to succeed")
	}
}

func TestCompressionMiddleware_SSEContentTypeBypassed(t *testing.T) {
	// SSE on a non-/api/events path is detected via Content-Type.
	// Body stays under threshold so it gets flushed out via Flush() → undecided bypass.
	flushed := false
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "data: hello\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("expected no Content-Encoding for SSE response")
	}
	if !flushed {
		t.Error("expected SSE Flush to succeed")
	}
}

func TestCompressionMiddleware_ContentLengthRemovedWhenCompressing(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "9999")
		_, _ = io.WriteString(w, body)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected gzip compression")
	}
	// Content-Length must be removed (chunked transfer will be used instead).
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("Content-Length should be absent when compressing, got %q", rec.Header().Get("Content-Length"))
	}
}

func TestCompressionMiddleware_StatusCodesPreserved(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, body)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected gzip for eligible 201 response")
	}
}

func TestCompressionMiddleware_FlusherPreserved(t *testing.T) {
	h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			panic("http.Flusher not available inside handler")
		}
		w.(http.Flusher).Flush()
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req) // must not panic
}

func TestCompressionMiddleware_WildcardAcceptEncoding(t *testing.T) {
	body := largeBody(10)
	h := compressionMiddleware(testOpts, makeHandler(body, "application/json"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "*")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("expected gzip with wildcard Accept-Encoding")
	}
}

func TestCompressionMiddleware_RoundTrip(t *testing.T) {
	bodies := []string{
		largeBody(5),
		largeBody(50),
		largeBody(500),
	}
	for _, original := range bodies {
		h := compressionMiddleware(testOpts, makeHandler(original, "application/json"))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		h.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("len=%d: expected gzip", len(original))
			continue
		}
		got := string(decompressGzip(t, rec.Body.Bytes()))
		if got != original {
			t.Errorf("len=%d: round-trip mismatch", len(original))
		}
	}
}

// -------- Fuzz --------

func FuzzAcceptsGzip(f *testing.F) {
	seeds := []string{
		"gzip",
		"gzip;q=0",
		"gzip, deflate",
		"gzip;q=1.0, deflate;q=0.8",
		"*",
		"identity",
		"",
		"xgzip",
		"gzip;q=bad",
		"gzip;q=0.000",
		"GZIP",
		"*, gzip;q=0",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, header string) {
		// Must never panic.
		_ = acceptsGzip(header)
	})
}

func FuzzCompressionRoundTrip(f *testing.F) {
	f.Add([]byte("hello world abcdefghij "))
	f.Add([]byte(`{"key":"value","n":42}`))
	f.Add(make([]byte, 128))

	f.Fuzz(func(t *testing.T, payload []byte) {
		// Payload must be at or above threshold.
		for len(payload) < minSize {
			payload = append(payload, payload...)
		}
		payload = payload[:max(minSize, len(payload))]

		h := compressionMiddleware(testOpts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		}))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		h.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Skip("no compression, skipping round-trip check")
		}
		got := decompressGzip(t, rec.Body.Bytes())
		if !bytes.Equal(got, payload) {
			t.Fatal("round-trip mismatch")
		}
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
