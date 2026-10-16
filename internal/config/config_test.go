package config

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
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
		"BUNGIE_API_KEY":             "k",
		"DATABASE_URL":               "postgres:///dqr?host=/tmp",
		"BUNGIE_OAUTH_CLIENT_ID":     "55818",
		"BUNGIE_OAUTH_CLIENT_SECRET": "secret",
		"BUNGIE_OAUTH_REDIRECT_URL":  "https://localhost:8443/auth/callback",
		"TOKEN_ENCRYPTION_KEY":       base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}
}

func TestServerFromEnv_Defaults(t *testing.T) {
	s, err := ServerFromEnv(envFrom(serverEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if s.BungieAPIKey != "k" || s.BungieBaseURL != "https://www.bungie.net" {
		t.Errorf("embedded config: api key=%q base url=%q", s.BungieAPIKey, s.BungieBaseURL)
	}
	if s.Addr != ":8443" || s.StatsBaseURL != "https://stats.bungie.net" {
		t.Errorf("addr=%q stats=%q", s.Addr, s.StatsBaseURL)
	}
	if len(s.TokenKey) != 32 {
		t.Errorf("token key length = %d, want 32", len(s.TokenKey))
	}
	if s.OAuthClientID != "55818" || s.OAuthClientSecret != "secret" {
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

func TestServerFromEnv_StringRedactsSecrets(t *testing.T) {
	s, err := ServerFromEnv(envFrom(serverEnv()))
	if err != nil {
		t.Fatal(err)
	}
	out := fmt.Sprintf("%v", s)
	for _, secret := range []string{"secret", "postgres:///dqr"} {
		if strings.Contains(out, secret) {
			t.Errorf("formatted server %q leaks %q", out, secret)
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
