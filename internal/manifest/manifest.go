// Package manifest holds the Destiny activity and activity-mode definitions needed to label player status.
package manifest

import (
	"context"
	"fmt"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

// ModeCategory is Bungie's DestinyActivityModeCategory.
type ModeCategory int

const (
	CategoryNone           ModeCategory = 0
	CategoryPvE            ModeCategory = 1
	CategoryPvP            ModeCategory = 2
	CategoryPvECompetitive ModeCategory = 3
)

func (c ModeCategory) String() string {
	switch c {
	case CategoryNone:
		return "None"
	case CategoryPvE:
		return "PvE"
	case CategoryPvP:
		return "PvP"
	case CategoryPvECompetitive:
		return "PvECompetitive"
	default:
		return fmt.Sprintf("ModeCategory(%d)", int(c))
	}
}

type Activity struct {
	Hash      uint32
	Name      string
	IsPvP     bool
	PlaceHash uint32
}

type Mode struct {
	Hash     uint32
	Name     string
	ModeType int
	Category ModeCategory
}

type Manifest struct {
	Version    string
	activities map[uint32]Activity
	modes      map[uint32]Mode
}

// Source is implemented by *bungie.Client.
type Source interface {
	GetManifestInfo(ctx context.Context) (bungie.ManifestInfo, error)
	GetFile(ctx context.Context, path string, out any) error
}

var _ Source = (*bungie.Client)(nil)

type displayProperties struct {
	Name string `json:"name"`
}

type activityDef struct {
	Hash              uint32            `json:"hash"`
	DisplayProperties displayProperties `json:"displayProperties"`
	IsPvP             bool              `json:"isPvP"`
	PlaceHash         uint32            `json:"placeHash"`
}

type modeDef struct {
	Hash                 uint32            `json:"hash"`
	DisplayProperties    displayProperties `json:"displayProperties"`
	ModeType             int               `json:"modeType"`
	ActivityModeCategory int               `json:"activityModeCategory"`
}

// Load downloads the current English activity and mode definitions.
func Load(ctx context.Context, src Source) (*Manifest, error) {
	info, err := src.GetManifestInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("manifest info: %w", err)
	}
	var acts map[string]activityDef
	if err := src.GetFile(ctx, info.ActivityDefPath, &acts); err != nil {
		return nil, fmt.Errorf("activity definitions: %w", err)
	}
	var modes map[string]modeDef
	if err := src.GetFile(ctx, info.ActivityModeDefPath, &modes); err != nil {
		return nil, fmt.Errorf("activity mode definitions: %w", err)
	}

	m := &Manifest{
		Version:    info.Version,
		activities: make(map[uint32]Activity, len(acts)),
		modes:      make(map[uint32]Mode, len(modes)),
	}
	for _, d := range acts {
		m.activities[d.Hash] = Activity{Hash: d.Hash, Name: d.DisplayProperties.Name, IsPvP: d.IsPvP, PlaceHash: d.PlaceHash}
	}
	for _, d := range modes {
		m.modes[d.Hash] = Mode{Hash: d.Hash, Name: d.DisplayProperties.Name, ModeType: d.ModeType, Category: ModeCategory(d.ActivityModeCategory)}
	}
	return m, nil
}

func (m *Manifest) Activity(hash uint32) (Activity, bool) {
	a, ok := m.activities[hash]
	return a, ok
}

func (m *Manifest) Mode(hash uint32) (Mode, bool) {
	mode, ok := m.modes[hash]
	return mode, ok
}
