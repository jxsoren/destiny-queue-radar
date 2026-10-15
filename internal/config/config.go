// Package config loads runtime configuration from the environment.
package config

import (
	"errors"
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
