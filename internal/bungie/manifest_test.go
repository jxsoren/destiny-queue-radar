package bungie

import (
	"context"
	"net/http"
	"testing"
)

func TestGetManifestInfo(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"version":"v123","jsonWorldComponentContentPaths":{"en":{
			"DestinyActivityDefinition":"/common/act.json",
			"DestinyActivityModeDefinition":"/common/mode.json"}}}}`))
	})
	info, err := c.GetManifestInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := ManifestInfo{Version: "v123", ActivityDefPath: "/common/act.json", ActivityModeDefPath: "/common/mode.json"}
	if info != want {
		t.Errorf("got %+v, want %+v", info, want)
	}
}

func TestGetFile_DecodesPlainJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/common/act.json" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"1":{"hash":1}}`))
	})
	var out map[string]struct {
		Hash uint32 `json:"hash"`
	}
	if err := c.GetFile(context.Background(), "/common/act.json", &out); err != nil {
		t.Fatal(err)
	}
	if out["1"].Hash != 1 {
		t.Errorf("out = %+v", out)
	}
}

func TestGetFile_Non200IsError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	var out map[string]any
	if err := c.GetFile(context.Background(), "/missing.json", &out); err == nil {
		t.Fatal("expected error")
	}
}
