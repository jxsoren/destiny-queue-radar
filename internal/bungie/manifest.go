package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ManifestInfo struct {
	Version             string
	ActivityDefPath     string
	ActivityModeDefPath string
}

type manifestResponse struct {
	Version string `json:"version"`
	Paths   map[string]struct {
		Activity     string `json:"DestinyActivityDefinition"`
		ActivityMode string `json:"DestinyActivityModeDefinition"`
	} `json:"jsonWorldComponentContentPaths"`
}

func (c *Client) GetManifestInfo(ctx context.Context) (ManifestInfo, error) {
	m, err := call[manifestResponse](ctx, c, http.MethodGet, "/Platform/Destiny2/Manifest/", nil)
	if err != nil {
		return ManifestInfo{}, err
	}
	en, ok := m.Paths["en"]
	if !ok || en.Activity == "" || en.ActivityMode == "" {
		return ManifestInfo{}, fmt.Errorf("bungie: manifest %s missing en activity definitions", m.Version)
	}
	return ManifestInfo{Version: m.Version, ActivityDefPath: en.Activity, ActivityModeDefPath: en.ActivityMode}, nil
}

// GetFile downloads a non-enveloped JSON file (e.g. a manifest definition) and decodes it into out.
func (c *Client) GetFile(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("bungie: build request %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return &TransportError{Path: path, Err: err}
	}
	defer func() {
		// Drain so the keep-alive connection can be reused.
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return &APIError{HTTPStatus: resp.StatusCode, Message: "fetch " + path}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("bungie: decode %s: %w", path, err)
	}
	return nil
}
