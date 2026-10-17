package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSetup_TextByDefault(t *testing.T) {
	var buf bytes.Buffer
	log, flush, err := Setup(envFrom(nil), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer flush()
	log.Info("hello", "k", "v")
	if !strings.Contains(buf.String(), "level=INFO msg=hello k=v") {
		t.Errorf("output = %q, want text format", buf.String())
	}
}

func TestSetup_JSON(t *testing.T) {
	var buf bytes.Buffer
	log, flush, err := Setup(envFrom(map[string]string{"LOG_FORMAT": "json"}), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer flush()
	log.Info("hello", "k", "v")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output %q is not JSON: %v", buf.String(), err)
	}
	if line["msg"] != "hello" || line["level"] != "INFO" || line["k"] != "v" {
		t.Errorf("line = %v", line)
	}
}

func TestSetup_RejectsUnknownFormat(t *testing.T) {
	_, _, err := Setup(envFrom(map[string]string{"LOG_FORMAT": "yaml"}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "LOG_FORMAT") {
		t.Errorf("err = %v, want a LOG_FORMAT error", err)
	}
}

func TestSetup_RejectsBadDSN(t *testing.T) {
	_, _, err := Setup(envFrom(map[string]string{"SENTRY_DSN": "not a dsn"}), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "sentry") {
		t.Errorf("err = %v, want a sentry init error", err)
	}
}

func TestSentryOptions_RailwayDefaults(t *testing.T) {
	o := sentryOptions(envFrom(map[string]string{
		"SENTRY_DSN":               "https://key@o1.ingest.sentry.io/1",
		"RAILWAY_GIT_COMMIT_SHA":   "abc123",
		"RAILWAY_ENVIRONMENT_NAME": "production",
	}))
	if o.Release != "abc123" || o.Environment != "production" || o.SendDefaultPII {
		t.Errorf("options: release=%q environment=%q pii=%t", o.Release, o.Environment, o.SendDefaultPII)
	}
}

func TestSentryOptions_ExplicitSettingsWin(t *testing.T) {
	o := sentryOptions(envFrom(map[string]string{
		"SENTRY_RELEASE":           "v1.2.3",
		"SENTRY_ENVIRONMENT":       "staging",
		"RAILWAY_GIT_COMMIT_SHA":   "abc123",
		"RAILWAY_ENVIRONMENT_NAME": "production",
	}))
	if o.Release != "v1.2.3" || o.Environment != "staging" {
		t.Errorf("options: release=%q environment=%q", o.Release, o.Environment)
	}
}

// secret redacts itself the way config.Server and auth.TokenResponse do.
type secret string

func (secret) LogValue() slog.Value { return slog.StringValue("<redacted>") }

func newTestHandler(t *testing.T) (*slog.Logger, *sentry.MockTransport, *bytes.Buffer) {
	t.Helper()
	tr := &sentry.MockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "https://key@o1.ingest.sentry.io/1", Transport: tr})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	next := slog.NewTextHandler(&buf, nil)
	return slog.New(newSentryHandler(next, sentry.NewHub(client, sentry.NewScope()))), tr, &buf
}

func TestSentryHandler_ErrorBecomesEvent(t *testing.T) {
	log, tr, buf := newTestHandler(t)
	log = log.With("component", "auth").WithGroup("req")

	log.Error("persist tokens failed", "err", errors.New("connection reset"), "user_id", 7, "token", secret("hunter2"))

	events := tr.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Message != "persist tokens failed" || ev.Level != sentry.LevelError {
		t.Errorf("message=%q level=%q", ev.Message, ev.Level)
	}
	if len(ev.Fingerprint) != 1 || ev.Fingerprint[0] != "persist tokens failed" {
		t.Errorf("fingerprint = %v, want the log message", ev.Fingerprint)
	}
	if len(ev.Exception) == 0 || ev.Exception[len(ev.Exception)-1].Value != "connection reset" {
		t.Errorf("exception = %+v", ev.Exception)
	}
	attrs := ev.Contexts["log"]
	if attrs["component"] != "auth" || attrs["req.user_id"] != int64(7) || attrs["req.token"] != "<redacted>" {
		t.Errorf("log context = %v", attrs)
	}
	if _, ok := attrs["req.err"]; ok {
		t.Error("err should be the exception, not repeated in the context")
	}
	if !strings.Contains(buf.String(), "persist tokens failed") {
		t.Errorf("next handler did not get the record: %q", buf.String())
	}
}

func TestSentryHandler_ErrorWithoutErrAttr(t *testing.T) {
	log, tr, _ := newTestHandler(t)
	log.Error("poller stalled")

	events := tr.Events()
	if len(events) != 1 || events[0].Message != "poller stalled" {
		t.Fatalf("events = %+v", events)
	}
}

func TestSentryHandler_BelowErrorIsOnlyLogged(t *testing.T) {
	log, tr, buf := newTestHandler(t)
	log.Warn("bungie degraded", "err", errors.New("slow"))
	log.Info("listening")

	if n := len(tr.Events()); n != 0 {
		t.Errorf("events = %d, want 0", n)
	}
	if !strings.Contains(buf.String(), "bungie degraded") || !strings.Contains(buf.String(), "listening") {
		t.Errorf("next handler output = %q", buf.String())
	}
}

func TestSentryHandler_RepeatsWithinAMinuteAreOnlyLogged(t *testing.T) {
	log, tr, buf := newTestHandler(t)
	for range 3 {
		log.Error("refresh active set", "err", errors.New("connection refused"))
	}
	log.Error("cleanup players", "err", errors.New("connection refused"))

	if n := len(tr.Events()); n != 2 {
		t.Errorf("events = %d, want 2 (one per message)", n)
	}
	if n := strings.Count(buf.String(), "refresh active set"); n != 3 {
		t.Errorf("logged %d times, want 3", n)
	}
}

func TestThrottle_AllowsAgainAfterInterval(t *testing.T) {
	th := newThrottle(time.Minute)
	t0 := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		at   time.Duration
		want bool
	}{{0, true}, {59 * time.Second, false}, {time.Minute, true}, {90 * time.Second, false}} {
		if got := th.allow("m", t0.Add(c.at)); got != c.want {
			t.Errorf("allow at +%s = %t, want %t", c.at, got, c.want)
		}
	}
}
