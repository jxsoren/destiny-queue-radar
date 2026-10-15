package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

type fakeSource struct {
	info  bungie.ManifestInfo
	files map[string]any
}

func (f fakeSource) GetManifestInfo(context.Context) (bungie.ManifestInfo, error) { return f.info, nil }

func (f fakeSource) GetFile(_ context.Context, path string, out any) error {
	b, err := json.Marshal(f.files[path])
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func newFake() fakeSource {
	return fakeSource{
		info: bungie.ManifestInfo{Version: "v1", ActivityDefPath: "/a.json", ActivityModeDefPath: "/m.json"},
		files: map[string]any{
			"/a.json": map[string]any{
				"100": map[string]any{"hash": 100, "displayProperties": map[string]any{"name": "Burnout"}, "isPvP": true, "placeHash": 7},
				"200": map[string]any{"hash": 200, "displayProperties": map[string]any{"name": "Orbit"}, "isPvP": false},
			},
			"/m.json": map[string]any{
				"300": map[string]any{"hash": 300, "displayProperties": map[string]any{"name": "Trials of Osiris"}, "modeType": 84, "activityModeCategory": 2},
				"400": map[string]any{"hash": 400, "displayProperties": map[string]any{"name": "Raid"}, "modeType": 4, "activityModeCategory": 1},
			},
		},
	}
}

func TestLoad_BuildsLookups(t *testing.T) {
	m, err := Load(context.Background(), newFake())
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "v1" {
		t.Errorf("version = %q", m.Version)
	}
	a, ok := m.Activity(100)
	if !ok || a.Name != "Burnout" || !a.IsPvP || a.PlaceHash != 7 {
		t.Errorf("activity = %+v ok=%v", a, ok)
	}
	mode, ok := m.Mode(300)
	if !ok || mode.Name != "Trials of Osiris" || mode.Category != CategoryPvP || mode.ModeType != 84 {
		t.Errorf("mode = %+v ok=%v", mode, ok)
	}
	if raid, _ := m.Mode(400); raid.Category != CategoryPvE {
		t.Errorf("raid category = %v", raid.Category)
	}
	if _, ok := m.Activity(999); ok {
		t.Error("unknown hash should not be found")
	}
}

func TestModeCategory_String(t *testing.T) {
	for cat, want := range map[ModeCategory]string{
		CategoryNone: "None", CategoryPvE: "PvE", CategoryPvP: "PvP", CategoryPvECompetitive: "PvECompetitive", 9: "ModeCategory(9)",
	} {
		if got := cat.String(); got != want {
			t.Errorf("%d: got %q want %q", int(cat), got, want)
		}
	}
}

var errBoom = errors.New("boom")

type failingSource struct {
	fakeSource
	infoErr, fileErr error
}

func (f failingSource) GetManifestInfo(ctx context.Context) (bungie.ManifestInfo, error) {
	if f.infoErr != nil {
		return bungie.ManifestInfo{}, f.infoErr
	}
	return f.fakeSource.GetManifestInfo(ctx)
}

func (f failingSource) GetFile(ctx context.Context, path string, out any) error {
	if f.fileErr != nil {
		return f.fileErr
	}
	return f.fakeSource.GetFile(ctx, path, out)
}

func TestLoad_ManifestInfoError(t *testing.T) {
	m, err := Load(context.Background(), failingSource{fakeSource: newFake(), infoErr: errBoom})
	if m != nil || !errors.Is(err, errBoom) {
		t.Errorf("m=%v err=%v", m, err)
	}
}

func TestLoad_ActivityFileError(t *testing.T) {
	m, err := Load(context.Background(), failingSource{fakeSource: newFake(), fileErr: errBoom})
	if m != nil || !errors.Is(err, errBoom) {
		t.Errorf("m=%v err=%v", m, err)
	}
}
