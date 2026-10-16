// Package config loads runtime configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const defaultBungieBaseURL = "https://www.bungie.net"

type Config struct {
	BungieAPIKey  string
	BungieBaseURL string
}

// FromEnv builds a Config using getenv (normally os.Getenv).
func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		BungieAPIKey:  getenv("BUNGIE_API_KEY"),
		BungieBaseURL: getenv("BUNGIE_BASE_URL"),
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
	TLSCertFile       string // optional; set together with TLSKeyFile
	TLSKeyFile        string
}

// ServerFromEnv builds the server configuration using getenv (normally os.Getenv).
func ServerFromEnv(getenv func(string) string) (Server, error) {
	base, err := FromEnv(getenv)
	if err != nil {
		return Server{}, err
	}
	s := Server{
		Config:            base,
		DatabaseURL:       getenv("DATABASE_URL"),
		OAuthClientID:     getenv("BUNGIE_OAUTH_CLIENT_ID"),
		OAuthClientSecret: getenv("BUNGIE_OAUTH_CLIENT_SECRET"),
		OAuthRedirectURL:  getenv("BUNGIE_OAUTH_REDIRECT_URL"),
		StatsBaseURL:      getenv("BUNGIE_STATS_BASE_URL"),
		Addr:              getenv("ADDR"),
		TLSCertFile:       getenv("TLS_CERT_FILE"),
		TLSKeyFile:        getenv("TLS_KEY_FILE"),
	}
	rawKey := getenv("TOKEN_ENCRYPTION_KEY")

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
	if err != nil || len(key) != 32 {
		return Server{}, errors.New("TOKEN_ENCRYPTION_KEY must be 32 bytes, base64-encoded (generate with: openssl rand -base64 32)")
	}
	s.TokenKey = key

	if (s.TLSCertFile == "") != (s.TLSKeyFile == "") {
		return Server{}, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if s.StatsBaseURL == "" {
		s.StatsBaseURL = defaultStatsBaseURL
	}
	s.StatsBaseURL = strings.TrimRight(s.StatsBaseURL, "/")
	if s.Addr == "" {
		s.Addr = defaultAddr
	}
	return s, nil
}
