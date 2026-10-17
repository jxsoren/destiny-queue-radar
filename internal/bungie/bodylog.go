package bungie

import (
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// NewBodyLogTransport wraps next (http.DefaultTransport when nil) to log each request's method,
// path, query, status, duration and the first maxBody bytes of its request and response bodies.
// Headers are never logged: they carry the API key and users' bearer tokens. Response bodies
// stream through; only the logged prefix is kept in memory.
func NewBodyLogTransport(next http.RoundTripper, log *slog.Logger, maxBody int) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &bodyLogTransport{next: next, log: log, max: maxBody}
}

type bodyLogTransport struct {
	next http.RoundTripper
	log  *slog.Logger
	max  int
}

func (t *bodyLogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attrs := []any{"method", req.Method, "path", req.URL.Path}
	if req.URL.RawQuery != "" {
		attrs = append(attrs, "query", req.URL.RawQuery)
	}
	if body, truncated, ok := t.requestBody(req); ok {
		attrs = append(attrs, "request_body", body)
		if truncated {
			attrs = append(attrs, "request_truncated", true)
		}
	}
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		t.log.InfoContext(req.Context(), "bungie http", append(attrs, "duration_ms", time.Since(start).Milliseconds(), "err", err)...)
		return nil, err
	}
	attrs = append(attrs, "status", resp.StatusCode)
	resp.Body = &loggedBody{ReadCloser: resp.Body, max: t.max, done: func(body []byte, truncated bool) {
		a := append(attrs, "duration_ms", time.Since(start).Milliseconds(), "response_body", string(body))
		if truncated {
			a = append(a, "response_truncated", true)
		}
		t.log.InfoContext(req.Context(), "bungie http", a...)
	}}
	return resp, nil
}

// requestBody reads up to max bytes of a copy of req's body, leaving req.Body untouched.
func (t *bodyLogTransport) requestBody(req *http.Request) (body string, truncated, ok bool) {
	if req.Body == nil || req.Body == http.NoBody || req.GetBody == nil {
		return "", false, false
	}
	rc, err := req.GetBody()
	if err != nil {
		return "", false, false
	}
	defer rc.Close()
	b, _ := io.ReadAll(io.LimitReader(rc, int64(t.max)+1))
	if len(b) > t.max {
		return string(b[:t.max]), true, true
	}
	return string(b), false, true
}

// loggedBody keeps the first max bytes read through it and reports them once, on Close.
type loggedBody struct {
	io.ReadCloser
	max  int
	buf  []byte
	read int
	once sync.Once
	done func(body []byte, truncated bool)
}

func (b *loggedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if room := b.max - len(b.buf); room > 0 {
		b.buf = append(b.buf, p[:min(n, room)]...)
	}
	b.read += n
	return n, err
}

func (b *loggedBody) Close() error {
	b.once.Do(func() { b.done(b.buf, b.read > len(b.buf)) })
	return b.ReadCloser.Close()
}
