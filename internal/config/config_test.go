package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const (
	testAPIKey       = "APIKEY-test-9c2d"
	testClientSecret = "client-secret-x91"
	testDatabaseURL  = "postgres:///dqr?host=/tmp"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestFromEnv_DefaultsBaseURL(t *testing.T) {
	c, err := FromEnv(envFrom(map[string]string{"BUNGIE_API_KEY": "k"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BungieAPIKey != "k" {
		t.Errorf("api key = %q", c.BungieAPIKey)
	}
	if c.BungieBaseURL != "https://www.bungie.net" {
		t.Errorf("base url = %q", c.BungieBaseURL)
	}
}

func TestFromEnv_TrimsTrailingSlash(t *testing.T) {
	c, err := FromEnv(envFrom(map[string]string{
		"BUNGIE_API_KEY":  "k",
		"BUNGIE_BASE_URL": "http://localhost:9999/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.BungieBaseURL != "http://localhost:9999" {
		t.Errorf("base url = %q", c.BungieBaseURL)
	}
}

func TestFromEnv_RequiresAPIKey(t *testing.T) {
	if _, err := FromEnv(envFrom(map[string]string{})); err == nil {
		t.Fatal("expected error when BUNGIE_API_KEY is missing")
	}
}

func TestFromEnv_WhitespaceOnlyAPIKeyIsMissing(t *testing.T) {
	if _, err := FromEnv(envFrom(map[string]string{"BUNGIE_API_KEY": "  \n"})); err == nil {
		t.Fatal("expected error when BUNGIE_API_KEY is whitespace only")
	}
}

func serverEnv() map[string]string {
	return map[string]string{
		"BUNGIE_API_KEY":             testAPIKey,
		"DATABASE_URL":               testDatabaseURL,
		"BUNGIE_OAUTH_CLIENT_ID":     "55818",
		"BUNGIE_OAUTH_CLIENT_SECRET": testClientSecret,
		"BUNGIE_OAUTH_REDIRECT_URL":  "https://localhost:8443/auth/callback",
		"TOKEN_ENCRYPTION_KEY":       base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}
}

func TestServerFromEnv_Defaults(t *testing.T) {
	s, err := ServerFromEnv(envFrom(serverEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if s.BungieAPIKey != testAPIKey || s.BungieBaseURL != "https://www.bungie.net" {
		t.Errorf("embedded config: api key=%q base url=%q", s.BungieAPIKey, s.BungieBaseURL)
	}
	if s.Addr != ":8443" || s.StatsBaseURL != "https://stats.bungie.net" {
		t.Errorf("addr=%q stats=%q", s.Addr, s.StatsBaseURL)
	}
	if len(s.TokenKey) != 32 {
		t.Errorf("token key length = %d, want 32", len(s.TokenKey))
	}
	if s.OAuthClientID != "55818" || s.OAuthClientSecret != testClientSecret {
		t.Errorf("oauth client id=%q secret=%q", s.OAuthClientID, s.OAuthClientSecret)
	}
}

func TestServerFromEnv_ListsEveryMissingSetting(t *testing.T) {
	_, err := ServerFromEnv(envFrom(map[string]string{"BUNGIE_API_KEY": "k"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, name := range []string{"DATABASE_URL", "BUNGIE_OAUTH_CLIENT_ID", "BUNGIE_OAUTH_CLIENT_SECRET", "BUNGIE_OAUTH_REDIRECT_URL", "TOKEN_ENCRYPTION_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
}

func TestServerFromEnv_RejectsBadTokenKey(t *testing.T) {
	for _, key := range []string{
		"not base64!",
		base64.StdEncoding.EncodeToString(make([]byte, 16)),
		base64.StdEncoding.EncodeToString(make([]byte, 33)),
		base64.StdEncoding.EncodeToString(make([]byte, 64)),
	} {
		env := serverEnv()
		env["TOKEN_ENCRYPTION_KEY"] = key
		_, err := ServerFromEnv(envFrom(env))
		if err == nil {
			t.Errorf("key %q: expected error", key)
			continue
		}
		if strings.Contains(err.Error(), key) {
			t.Errorf("error %q leaks the key value", err)
		}
	}
}

func TestServerFromEnv_TrimsWhitespace(t *testing.T) {
	env := serverEnv()
	env["TOKEN_ENCRYPTION_KEY"] = base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n "
	s, err := ServerFromEnv(envFrom(env))
	if err != nil {
		t.Fatalf("key with trailing whitespace should decode: %v", err)
	}
	if len(s.TokenKey) != 32 {
		t.Errorf("token key length = %d, want 32", len(s.TokenKey))
	}

	env = serverEnv()
	env["DATABASE_URL"] = "   "
	_, err = ServerFromEnv(envFrom(env))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("whitespace-only DATABASE_URL should be reported missing, got %v", err)
	}
}

func TestServerFromEnv_RedactsSecretsInAllFormats(t *testing.T) {
	s, err := ServerFromEnv(envFrom(serverEnv()))
	if err != nil {
		t.Fatal(err)
	}
	secrets := []string{testAPIKey, testClientSecret, testDatabaseURL}

	formatted := map[string]string{
		"%v":        fmt.Sprintf("%v", s),
		"%#v":       fmt.Sprintf("%#v", s),
		"%+v":       fmt.Sprintf("%+v", s),
		"Config %v": fmt.Sprintf("%v", s.Config),
	}
	for verb, out := range formatted {
		for _, secret := range secrets {
			if strings.Contains(out, secret) {
				t.Errorf("%s output %q leaks %q", verb, out, secret)
			}
		}
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("loaded", slog.Any("cfg", s), slog.Any("bungie", s.Config))
	for _, secret := range secrets {
		if strings.Contains(buf.String(), secret) {
			t.Errorf("slog JSON output %q leaks %q", buf.String(), secret)
		}
	}
}

func TestServerFromEnv_TLSFilesMustBePaired(t *testing.T) {
	env := serverEnv()
	env["TLS_CERT_FILE"] = "certs/localhost.pem"
	if _, err := ServerFromEnv(envFrom(env)); err == nil {
		t.Error("cert without key should fail")
	}
	env["TLS_KEY_FILE"] = "certs/localhost-key.pem"
	s, err := ServerFromEnv(envFrom(env))
	if err != nil || s.TLSCertFile == "" || s.TLSKeyFile == "" {
		t.Errorf("paired TLS files: cert=%q key=%q err=%v", s.TLSCertFile, s.TLSKeyFile, err)
	}
}

func TestServerFromEnv_RequiresAPIKey(t *testing.T) {
	env := serverEnv()
	delete(env, "BUNGIE_API_KEY")
	if _, err := ServerFromEnv(envFrom(env)); err == nil {
		t.Error("expected error without BUNGIE_API_KEY")
	}
}

func TestServerFromEnv_AddrFromPort(t *testing.T) {
	env := serverEnv()
	env["PORT"] = "8080"
	s, err := ServerFromEnv(envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if s.Addr != ":8080" {
		t.Errorf("addr = %q, want :8080", s.Addr)
	}
}

func TestServerFromEnv_AddrWinsOverPort(t *testing.T) {
	env := serverEnv()
	env["PORT"] = "8080"
	env["ADDR"] = "127.0.0.1:9000"
	s, err := ServerFromEnv(envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if s.Addr != "127.0.0.1:9000" {
		t.Errorf("addr = %q, want 127.0.0.1:9000", s.Addr)
	}
}

func TestServerFromEnv_BungieLogBodyBytes(t *testing.T) {
	s, err := ServerFromEnv(envFrom(serverEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if s.BungieLogBodyBytes != 0 {
		t.Errorf("default = %d, want 0 (off)", s.BungieLogBodyBytes)
	}

	env := serverEnv()
	env["BUNGIE_LOG_BODY_BYTES"] = "16384"
	if s, err = ServerFromEnv(envFrom(env)); err != nil || s.BungieLogBodyBytes != 16384 {
		t.Errorf("got %d, %v; want 16384", s.BungieLogBodyBytes, err)
	}

	for _, bad := range []string{"-1", "lots"} {
		env["BUNGIE_LOG_BODY_BYTES"] = bad
		if _, err := ServerFromEnv(envFrom(env)); err == nil || !strings.Contains(err.Error(), "BUNGIE_LOG_BODY_BYTES") {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}
