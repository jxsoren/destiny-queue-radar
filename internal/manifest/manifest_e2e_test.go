package manifest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
)

func TestLoad_EndToEndAgainstBungieShapes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/Platform/Destiny2/Manifest/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Response":{"version":"250101.1","mobileAssetContentPath":"/x",
			"jsonWorldComponentContentPaths":{"en":{
				"DestinyActivityDefinition":"/common/destiny2_content/json/en/DestinyActivityDefinition-abc.json",
				"DestinyActivityModeDefinition":"/common/destiny2_content/json/en/DestinyActivityModeDefinition-def.json",
				"DestinyClassDefinition":"/common/other.json"}}},
			"ErrorCode":1,"ThrottleSeconds":0,"ErrorStatus":"Success","Message":"Ok","MessageData":{}}`))
	})
	mux.HandleFunc("/common/destiny2_content/json/en/DestinyActivityDefinition-abc.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"2693136600":{"displayProperties":{"description":"d","name":"Rumble","hasIcon":true},
			"isPvP":true,"placeHash":4088006058,"hash":2693136600,"index":5,"redacted":false,"blacklisted":false}}`))
	})
	mux.HandleFunc("/common/destiny2_content/json/en/DestinyActivityModeDefinition-def.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"1164760504":{"displayProperties":{"name":"Crucible","hasIcon":false},
			"pgcrImage":"/img.jpg","modeType":5,"activityModeCategory":2,"isTeamBased":true,"hash":1164760504,"index":3}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m, err := manifest.Load(context.Background(), bungie.NewClient("k", srv.URL, srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "250101.1" {
		t.Errorf("version = %q", m.Version)
	}
	a, ok := m.Activity(2693136600)
	if !ok || a.Name != "Rumble" || !a.IsPvP || a.PlaceHash != 4088006058 {
		t.Errorf("activity = %+v ok=%v", a, ok)
	}
	mode, ok := m.Mode(1164760504)
	if !ok || mode.Name != "Crucible" || mode.ModeType != 5 || mode.Category != manifest.CategoryPvP {
		t.Errorf("mode = %+v ok=%v", mode, ok)
	}
}
