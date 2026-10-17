// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

const defaultBungieBaseURL = "https://www.bungie.net"

type Config struct {
	BungieAPIKey  string
	BungieBaseURL string
}

// String summarizes the Bungie settings with the API key redacted.
func (c Config) String() string {
	return fmt.Sprintf("Config{BungieAPIKey:<redacted> BungieBaseURL:%s}", c.BungieBaseURL)
}

// GoString matches String so %#v cannot leak the API key.
func (c Config) GoString() string { return c.String() }

// LogValue keeps slog output redacted.
func (c Config) LogValue() slog.Value { return slog.StringValue(c.String()) }

// FromEnv builds a Config using getenv (normally os.Getenv).
func FromEnv(getenv func(string) string) (Config, error) {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	c := Config{
		BungieAPIKey:  get("BUNGIE_API_KEY"),
		BungieBaseURL: get("BUNGIE_BASE_URL"),
	}
	if c.BungieAPIKey == "" {
		return Config{}, errors.New("BUNGIE_API_KEY is required")
	}
	if c.BungieBaseURL == "" {
		c.BungieBaseURL = defaultBungieBaseURL
	}
	c.BungieBaseURL = strings.TrimRight(c.BungieBaseURL, "/")
	return c, nil
}

const (
	defaultStatsBaseURL = "https://stats.bungie.net"
	defaultAddr         = ":8443"
)

// Server is the configuration for cmd/server.
type Server struct {
	Config
	DatabaseURL       string
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURL  string
	TokenKey          []byte // AES-256 key for OAuth tokens at rest
	StatsBaseURL      string // host for post-game carnage reports
	Addr              string
	Port              string // used for Addr when ADDR is unset; set by hosts such as Railway
	TLSCertFile       string // optional; set together with TLSKeyFile
	TLSKeyFile        string
	// BungieLogBodyBytes, when > 0, logs every Bungie Platform request with up to this many
	// bytes of its request and response bodies. Off (0) by default.
	BungieLogBodyBytes int
}

// String summarizes the configuration with secrets and the database URL redacted.
func (s Server) String() string {
	tls := s.TLSCertFile != "" && s.TLSKeyFile != ""
	return fmt.Sprintf("Server{Addr:%s DatabaseURL:<set> OAuthClientID:%s OAuthClientSecret:<redacted> TokenKey:<redacted> TLS:%t}",
		s.Addr, s.OAuthClientID, tls)
}

// GoString matches String so %#v cannot leak secrets.
func (s Server) GoString() string { return s.String() }

// LogValue keeps slog output redacted.
func (s Server) LogValue() slog.Value { return slog.StringValue(s.String()) }

// ServerFromEnv builds the server configuration using getenv (normally os.Getenv).
func ServerFromEnv(getenv func(string) string) (Server, error) {
	base, err := FromEnv(getenv)
	if err != nil {
		return Server{}, err
	}
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	s := Server{
		Config:            base,
		DatabaseURL:       get("DATABASE_URL"),
		OAuthClientID:     get("BUNGIE_OAUTH_CLIENT_ID"),
		OAuthClientSecret: get("BUNGIE_OAUTH_CLIENT_SECRET"),
		OAuthRedirectURL:  get("BUNGIE_OAUTH_REDIRECT_URL"),
		StatsBaseURL:      get("BUNGIE_STATS_BASE_URL"),
		Addr:              get("ADDR"),
		Port:              get("PORT"),
		TLSCertFile:       get("TLS_CERT_FILE"),
		TLSKeyFile:        get("TLS_KEY_FILE"),
	}
	rawKey := get("TOKEN_ENCRYPTION_KEY")

	required := []struct{ name, value string }{
		{"DATABASE_URL", s.DatabaseURL},
		{"BUNGIE_OAUTH_CLIENT_ID", s.OAuthClientID},
		{"BUNGIE_OAUTH_CLIENT_SECRET", s.OAuthClientSecret},
		{"BUNGIE_OAUTH_REDIRECT_URL", s.OAuthRedirectURL},
		{"TOKEN_ENCRYPTION_KEY", rawKey},
	}
	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return Server{}, fmt.Errorf("missing required settings: %s", strings.Join(missing, ", "))
	}

	key, err := base64.StdEncoding.DecodeString(rawKey)
	// cryptobox.New also enforces the 32-byte length (defence in depth).
	if err != nil || len(key) != 32 {
		return Server{}, errors.New("TOKEN_ENCRYPTION_KEY must be 32 bytes, base64-encoded (generate with: openssl rand -base64 32)")
	}
	s.TokenKey = key

	if (s.TLSCertFile == "") != (s.TLSKeyFile == "") {
		return Server{}, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if raw := get("BUNGIE_LOG_BODY_BYTES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return Server{}, errors.New("BUNGIE_LOG_BODY_BYTES must be a non-negative number of bytes")
		}
		s.BungieLogBodyBytes = n
	}
	if s.StatsBaseURL == "" {
		s.StatsBaseURL = defaultStatsBaseURL
	}
	s.StatsBaseURL = strings.TrimRight(s.StatsBaseURL, "/")
	if s.Addr == "" && s.Port != "" {
		s.Addr = ":" + s.Port
	}
	if s.Addr == "" {
		s.Addr = defaultAddr
	}
	return s, nil
}
