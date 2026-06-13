package httpapi

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type compressionOptions struct {
	MinimumSize int
	GzipLevel   int
}

var gzipPool sync.Pool

type compressionState uint8

const (
	undecided   compressionState = iota
	compressing compressionState = iota
	bypassed    compressionState = iota
)

// compressionResponseWriter buffers response bytes until it can decide
// whether to compress. Once the buffer reaches MinimumSize and the
// content type is eligible, it transparently switches to gzip. Responses
// that end before the threshold are forwarded uncompressed.
type compressionResponseWriter struct {
	http.ResponseWriter
	opts            compressionOptions
	buf             []byte
	statusCode      int
	state           compressionState
	gz              *gzip.Writer
	headerForwarded bool
}

func (c *compressionResponseWriter) Header() http.Header {
	return c.ResponseWriter.Header()
}

func (c *compressionResponseWriter) WriteHeader(code int) {
	if c.headerForwarded {
		return
	}
	c.statusCode = code
	// Immediate bypass: no body for these status codes.
	if code == http.StatusNoContent || code == http.StatusNotModified || code < 200 {
		c.forwardBypass()
	}
}

func (c *compressionResponseWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	switch c.state {
	case bypassed:
		if !c.headerForwarded {
			c.forwardBypass()
		}
		return c.ResponseWriter.Write(b)
	case compressing:
		return c.gz.Write(b)
	case undecided:
		if c.headerForwarded {
			// Was bypassed via WriteHeader (e.g. 204) before any Write.
			return c.ResponseWriter.Write(b)
		}
		c.buf = append(c.buf, b...)
		if len(c.buf) >= c.opts.MinimumSize {
			if err := c.decide(); err != nil {
				return 0, err
			}
		}
		return len(b), nil
	}
	return len(b), nil
}

// decide checks eligibility once the buffer crosses MinimumSize.
func (c *compressionResponseWriter) decide() error {
	h := c.ResponseWriter.Header()

	if h.Get("Content-Encoding") != "" {
		c.forwardBypassWithBuf()
		return nil
	}
	if h.Get("Content-Range") != "" {
		c.forwardBypassWithBuf()
		return nil
	}

	ct := h.Get("Content-Type")
	if ct == "" && len(c.buf) > 0 {
		ct = http.DetectContentType(c.buf)
		h.Set("Content-Type", ct)
	}

	if strings.Contains(strings.ToLower(ct), "text/event-stream") {
		c.forwardBypassWithBuf()
		return nil
	}
	if !isCompressible(ct) {
		c.forwardBypassWithBuf()
		return nil
	}

	// Transition to compressing.
	c.state = compressing
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	addVary(h, "Accept-Encoding")

	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	c.ResponseWriter.WriteHeader(c.statusCode)
	c.headerForwarded = true

	c.gz = acquireGzipWriter(c.ResponseWriter, c.opts.GzipLevel)
	if len(c.buf) > 0 {
		if _, err := c.gz.Write(c.buf); err != nil {
			return err
		}
		c.buf = nil
	}
	return nil
}

func (c *compressionResponseWriter) forwardBypass() {
	c.state = bypassed
	if !c.headerForwarded {
		if c.statusCode == 0 {
			c.statusCode = http.StatusOK
		}
		c.ResponseWriter.WriteHeader(c.statusCode)
		c.headerForwarded = true
	}
}

func (c *compressionResponseWriter) forwardBypassWithBuf() {
	c.forwardBypass()
	if len(c.buf) > 0 {
		_, _ = c.ResponseWriter.Write(c.buf)
		c.buf = nil
	}
}

func (c *compressionResponseWriter) close() {
	if c.state == undecided {
		c.forwardBypassWithBuf()
		return
	}
	if c.state == compressing && c.gz != nil {
		c.gz.Close()
		gzipPool.Put(c.gz)
		c.gz = nil
	}
	if !c.headerForwarded {
		c.forwardBypass()
	}
}

// Flush implements http.Flusher so SSE streams through the middleware.
func (c *compressionResponseWriter) Flush() {
	switch c.state {
	case compressing:
		if c.gz != nil {
			_ = c.gz.Flush()
		}
		if f, ok := c.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	case bypassed:
		if f, ok := c.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	case undecided:
		// Force bypass so SSE frames not yet at threshold are sent immediately.
		c.forwardBypassWithBuf()
		if f, ok := c.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// ReadFrom allows efficient file serving when bypassed.
func (c *compressionResponseWriter) ReadFrom(src io.Reader) (int64, error) {
	if c.state == bypassed {
		if rf, ok := c.ResponseWriter.(io.ReaderFrom); ok {
			return rf.ReadFrom(src)
		}
	}
	// Wrap c in a plain io.Writer to prevent io.Copy from detecting the
	// ReaderFrom interface and calling ReadFrom again (infinite recursion).
	return io.Copy(struct{ io.Writer }{c}, src)
}

func acquireGzipWriter(w io.Writer, level int) *gzip.Writer {
	if gz, ok := gzipPool.Get().(*gzip.Writer); ok {
		gz.Reset(w)
		return gz
	}
	gz, _ := gzip.NewWriterLevel(w, level)
	return gz
}

// compressionMiddleware wraps next with transparent gzip negotiation.
// SSE endpoints, range requests, and clients without gzip support bypass compression.
func compressionMiddleware(opts compressionOptions, next http.Handler) http.Handler {
	if opts.MinimumSize <= 0 {
		opts.MinimumSize = 1024
	}
	if opts.GzipLevel <= 0 {
		opts.GzipLevel = gzip.BestSpeed
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE endpoint: bypass but still advertise Vary.
		if r.URL.Path == "/api/events" {
			w.Header().Add("Vary", "Accept-Encoding")
			next.ServeHTTP(w, r)
			return
		}
		// Range requests bypass compression.
		if r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		crw := &compressionResponseWriter{
			ResponseWriter: w,
			opts:           opts,
		}
		defer crw.close()
		next.ServeHTTP(crw, r)
	})
}

// acceptsGzip reports whether the Accept-Encoding header value permits gzip
// with non-zero quality. It handles the wildcard (*), quality parameters,
// and explicit gzip;q=0 overriding a wildcard.
func acceptsGzip(header string) bool {
	if header == "" {
		return false
	}
	gzipQ := -1.0
	wildcardQ := -1.0

	for _, token := range strings.Split(header, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		coding, q := parseEncodingToken(token)
		switch strings.ToLower(coding) {
		case "gzip":
			gzipQ = q
		case "*":
			wildcardQ = q
		}
	}

	if gzipQ >= 0 {
		return gzipQ > 0
	}
	if wildcardQ >= 0 {
		return wildcardQ > 0
	}
	return false
}

// parseEncodingToken splits "coding;q=value" returning the coding name and
// its quality (defaulting to 1.0). Malformed quality values are treated as 0.
func parseEncodingToken(token string) (coding string, q float64) {
	parts := strings.SplitN(token, ";", 2)
	coding = strings.TrimSpace(parts[0])
	q = 1.0
	if len(parts) < 2 {
		return
	}
	param := strings.TrimSpace(parts[1])
	if !strings.HasPrefix(strings.ToLower(param), "q=") {
		return
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(param[2:]), 64)
	if err != nil || v < 0 {
		q = 0
	} else {
		q = v
	}
	return
}

// isCompressible reports whether the Content-Type value represents a format
// that benefits from gzip. Already-compressed formats (images, archives, etc.) return false.
func isCompressible(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if ct == "" {
		return false
	}
	for _, prefix := range compressiblePrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

var compressiblePrefixes = []string{
	"text/",
	"application/json",
	"application/javascript",
	"application/x-javascript",
	"application/xml",
	"application/xhtml+xml",
	"application/atom+xml",
	"application/rss+xml",
	"image/svg+xml",
}

// addVary appends value to the Vary header if not already present.
func addVary(h http.Header, value string) {
	existing := h.Get("Vary")
	if existing == "" {
		h.Set("Vary", value)
		return
	}
	for _, v := range strings.Split(existing, ",") {
		if strings.EqualFold(strings.TrimSpace(v), value) {
			return
		}
	}
	h.Set("Vary", existing+", "+value)
}
