// Package logging builds the server's logger: text or JSON on stdout, and, when SENTRY_DSN is
// set, Error records also reported to Sentry as events.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	flushTimeout = 2 * time.Second
	// eventEvery limits Sentry events to one per log message per interval. A database outage
	// makes the poller log the same error every second; the log keeps all of them.
	eventEvery = time.Minute
)

// Setup builds the logger from the environment (getenv is normally os.Getenv). LOG_FORMAT is
// "text" (the default) or "json". The returned flush sends buffered Sentry events; call it
// before the process exits.
func Setup(getenv func(string) string, w io.Writer) (*slog.Logger, func(), error) {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	var h slog.Handler
	switch get("LOG_FORMAT") {
	case "", "text":
		h = slog.NewTextHandler(w, nil)
	case "json":
		h = slog.NewJSONHandler(w, nil)
	default:
		return nil, nil, errors.New(`LOG_FORMAT must be "text" or "json"`)
	}
	if get("SENTRY_DSN") == "" {
		return slog.New(h), func() {}, nil
	}
	if err := sentry.Init(sentryOptions(getenv)); err != nil {
		return nil, nil, fmt.Errorf("sentry init: %w", err)
	}
	flush := func() { sentry.Flush(flushTimeout) }
	return slog.New(newSentryHandler(h, sentry.CurrentHub())), flush, nil
}

// sentryOptions reads SENTRY_DSN, SENTRY_RELEASE and SENTRY_ENVIRONMENT, falling back to the
// commit and environment Railway sets; the SDK cannot detect a release in the container.
func sentryOptions(getenv func(string) string) sentry.ClientOptions {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := get(k); v != "" {
				return v
			}
		}
		return ""
	}
	return sentry.ClientOptions{
		Dsn:            get("SENTRY_DSN"),
		Release:        first("SENTRY_RELEASE", "RAILWAY_GIT_COMMIT_SHA"),
		Environment:    first("SENTRY_ENVIRONMENT", "RAILWAY_ENVIRONMENT_NAME"),
		SendDefaultPII: false,
	}
}

// sentryHandler passes every record to next and reports Error records to Sentry. Events are
// grouped by log message: the messages are fixed strings, while error text often holds IDs.
type sentryHandler struct {
	next   slog.Handler
	hub    *sentry.Hub
	limit  *throttle   // shared by handlers derived with WithAttrs and WithGroup
	attrs  []slog.Attr // from WithAttrs, keys already prefixed with their groups
	prefix string      // open groups, e.g. "req."
}

func newSentryHandler(next slog.Handler, hub *sentry.Hub) *sentryHandler {
	return &sentryHandler{next: next, hub: hub, limit: newThrottle(eventEvery)}
}

func (h *sentryHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelError || h.next.Enabled(ctx, l)
}

func (h *sentryHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError && h.limit.allow(r.Message, r.Time) {
		h.capture(r)
	}
	if !h.next.Enabled(ctx, r.Level) {
		return nil
	}
	return h.next.Handle(ctx, r)
}

func (h *sentryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.next = h.next.WithAttrs(attrs)
	c.attrs = append(append([]slog.Attr(nil), h.attrs...), prefixed(h.prefix, attrs)...)
	return &c
}

func (h *sentryHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.next = h.next.WithGroup(name)
	c.prefix = h.prefix + name + "."
	return &c
}

func (h *sentryHandler) capture(r slog.Record) {
	attrs := append([]slog.Attr(nil), h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, prefixed(h.prefix, []slog.Attr{a})...)
		return true
	})

	ev := sentry.NewEvent()
	ev.Level = sentry.LevelError
	ev.Message = r.Message
	ev.Fingerprint = []string{r.Message}
	ev.Timestamp = r.Time
	fields := sentry.Context{}
	var exc error
	for _, a := range attrs {
		if err, ok := a.Value.Any().(error); ok && exc == nil {
			exc = err
			continue
		}
		fields[a.Key] = a.Value.Any()
	}
	if exc != nil {
		ev.SetException(exc, 5)
	}
	if len(fields) > 0 {
		ev.Contexts["log"] = fields
	}
	h.hub.CaptureEvent(ev)
}

// prefixed resolves attrs (applying LogValue, so redaction holds) and flattens groups into
// dotted keys under prefix.
func prefixed(prefix string, attrs []slog.Attr) []slog.Attr {
	var out []slog.Attr
	for _, a := range attrs {
		v := a.Value.Resolve()
		if v.Kind() == slog.KindGroup {
			p := prefix
			if a.Key != "" {
				p += a.Key + "."
			}
			out = append(out, prefixed(p, v.Group())...)
			continue
		}
		if a.Key == "" {
			continue
		}
		out = append(out, slog.Attr{Key: prefix + a.Key, Value: v})
	}
	return out
}

// throttle allows each key at most once per interval.
type throttle struct {
	every time.Duration
	mu    sync.Mutex
	last  map[string]time.Time
}

func newThrottle(every time.Duration) *throttle {
	return &throttle{every: every, last: map[string]time.Time{}}
}

func (t *throttle) allow(key string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ok := t.last[key]; ok && now.Sub(last) < t.every {
		return false
	}
	t.last[key] = now
	return true
}
