package config

import "testing"

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
