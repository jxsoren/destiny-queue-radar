package bungie

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// logLines decodes buf's JSON log lines.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func newBodyLogClient(t *testing.T, maxBody int, handler http.HandlerFunc) (*Client, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	hc := &http.Client{Transport: NewBodyLogTransport(srv.Client().Transport, log, maxBody)}
	return NewClient("secret-api-key", srv.URL, hc), &buf
}

// newBodyLogClientWith is newBodyLogClient with wrap applied to the log handler.
func newBodyLogClientWith(t *testing.T, wrap func(slog.Handler) slog.Handler, handler http.HandlerFunc) (*Client, *bytes.Buffer) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	var buf bytes.Buffer
	log := slog.New(wrap(slog.NewJSONHandler(&buf, nil)))
	hc := &http.Client{Transport: NewBodyLogTransport(srv.Client().Transport, log, 1<<10)}
	return NewClient("k", srv.URL, hc), &buf
}

func TestBodyLog_LogsRequestAndResponseWithoutHeaders(t *testing.T) {
	c, buf := newBodyLogClient(t, 1<<10, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write([]byte(`{"Response":{"value":"hi"},"ErrorCode":1}`))
	})
	if _, err := callAs[echo](context.Background(), c, "user-bearer-token", http.MethodPost, "/Platform/Echo/", map[string]string{"q": "Guardian"}); err != nil {
		t.Fatal(err)
	}

	lines := logLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1: %s", len(lines), buf)
	}
	l := lines[0]
	if l["msg"] != "bungie http" || l["method"] != "POST" || l["path"] != "/Platform/Echo/" || l["status"] != float64(200) {
		t.Errorf("line = %v", l)
	}
	if l["request_body"] != `{"q":"Guardian"}` || l["response_body"] != `{"Response":{"value":"hi"},"ErrorCode":1}` {
		t.Errorf("bodies: request=%q response=%q", l["request_body"], l["response_body"])
	}
	if _, ok := l["duration_ms"]; !ok {
		t.Error("missing duration_ms")
	}
	if s := buf.String(); strings.Contains(s, "secret-api-key") || strings.Contains(s, "user-bearer-token") {
		t.Errorf("credentials leaked into log: %s", s)
	}
}

func TestBodyLog_TruncatesLargeBodiesButClientReadsAll(t *testing.T) {
	big := strings.Repeat("x", 10_000)
	c, buf := newBodyLogClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(big))
	})
	var got string
	_ = c.GetFile(context.Background(), "/common/big.json", &got) // not JSON; only the log matters

	l := logLines(t, buf)[0]
	body, _ := l["response_body"].(string)
	if body != strings.Repeat("x", 100) || l["response_truncated"] != true {
		t.Errorf("response_body len=%d truncated=%v", len(body), l["response_truncated"])
	}
}

func TestBodyLog_ClientStillReceivesFullBody(t *testing.T) {
	payload := `{"Response":{"value":"` + strings.Repeat("y", 5000) + `"},"ErrorCode":1}`
	c, _ := newBodyLogClient(t, 64, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(payload))
	})
	got, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Echo/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Value) != 5000 {
		t.Errorf("value length = %d, want 5000", len(got.Value))
	}
}

func TestBodyLog_LogsTransportErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused
	var buf bytes.Buffer
	hc := &http.Client{Transport: NewBodyLogTransport(http.DefaultTransport, slog.New(slog.NewJSONHandler(&buf, nil)), 100)}
	c := NewClient("k", url, hc)
	c.maxRetries = 0

	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Echo/", nil); err == nil {
		t.Fatal("expected error")
	}
	l := logLines(t, &buf)[0]
	if l["path"] != "/Platform/Echo/" || l["err"] == nil {
		t.Errorf("line = %v", l)
	}
}

func TestBodyLog_PassesTheRequestContextToTheLogger(t *testing.T) {
	c, buf := newBodyLogClientWith(t, func(h slog.Handler) slog.Handler { return ctxProbe{h} }, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Response":{},"ErrorCode":1}`))
	})
	ctx := context.WithValue(context.Background(), probeKey{}, "from-ctx")
	if _, err := call[echo](ctx, c, http.MethodGet, "/Platform/Echo/", nil); err != nil {
		t.Fatal(err)
	}
	if l := logLines(t, buf)[0]; l["probe"] != "from-ctx" {
		t.Errorf("line = %v, want the request context reaching the handler", l)
	}
}

type probeKey struct{}

// ctxProbe copies probeKey from the context onto the record, as logging.WithRequestID does.
type ctxProbe struct{ slog.Handler }

func (p ctxProbe) Handle(ctx context.Context, r slog.Record) error {
	if v, ok := ctx.Value(probeKey{}).(string); ok {
		r.AddAttrs(slog.String("probe", v))
	}
	return p.Handler.Handle(ctx, r)
}
