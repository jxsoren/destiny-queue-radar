package bungie

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ModeAllPvP is DestinyActivityModeType.AllPvP.
const ModeAllPvP = 5

type ActivityDetails struct {
	ReferenceID          uint32 `json:"referenceId"`          // the map
	DirectorActivityHash uint32 `json:"directorActivityHash"` // the playlist
	InstanceID           string `json:"instanceId"`
	Mode                 int    `json:"mode"`
	Modes                []int  `json:"modes"`
}

type HistoryActivity struct {
	Period          time.Time            `json:"period"` // when the activity started
	ActivityDetails ActivityDetails      `json:"activityDetails"`
	Values          map[string]PGCRValue `json:"values"`
}

// Duration is how long the player was in the activity (values.activityDurationSeconds), or 0
// when Bungie did not report it.
func (a HistoryActivity) Duration() time.Duration {
	v, ok := a.Values["activityDurationSeconds"]
	if !ok || v.Basic.Value <= 0 {
		return 0
	}
	return time.Duration(v.Basic.Value * float64(time.Second))
}

// EndedAt is when the player's activity ended: its start plus its duration. Without a reported
// duration it is the start, the latest time known for certain.
func (a HistoryActivity) EndedAt() time.Time { return a.Period.Add(a.Duration()) }

// ActivityHistory returns one character's most recent activities in the given mode.
// token may be empty for players whose history is public.
func (c *Client) ActivityHistory(ctx context.Context, token string, membershipType int64, membershipID, characterID string, mode, count int) ([]HistoryActivity, error) {
	path := fmt.Sprintf("/Platform/Destiny2/%d/Account/%s/Character/%s/Stats/Activities/?mode=%d&count=%d",
		membershipType, membershipID, characterID, mode, count)
	r, err := callAs[struct {
		Activities []HistoryActivity `json:"activities"`
	}](ctx, c, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return r.Activities, nil
}

type PGCRUserInfo struct {
	UserInfoCard
	IconPath string `json:"iconPath"`
}

type PGCRPlayer struct {
	DestinyUserInfo PGCRUserInfo `json:"destinyUserInfo"`
	CharacterClass  string       `json:"characterClass"`
}

type PGCRValue struct {
	Basic struct {
		Value        float64 `json:"value"`
		DisplayValue string  `json:"displayValue"`
	} `json:"basic"`
}

type PGCREntry struct {
	CharacterID string               `json:"characterId"`
	Player      PGCRPlayer           `json:"player"`
	Values      map[string]PGCRValue `json:"values"`
}

// Team returns the entry's team number, or -1 when the activity has no teams.
func (e PGCREntry) Team() int {
	v, ok := e.Values["team"]
	if !ok {
		return -1
	}
	return int(v.Basic.Value)
}

// PGCR is a post-game carnage report: everyone who played one activity instance.
type PGCR struct {
	Period          time.Time       `json:"period"`
	ActivityDetails ActivityDetails `json:"activityDetails"`
	Entries         []PGCREntry     `json:"entries"`
}

// GetPGCR fetches a post-game carnage report. Call it on a Client whose base URL is the stats
// host (https://stats.bungie.net).
func (c *Client) GetPGCR(ctx context.Context, instanceID string) (*PGCR, error) {
	r, err := call[PGCR](ctx, c, http.MethodGet,
		fmt.Sprintf("/Platform/Destiny2/Stats/PostGameCarnageReport/%s/", instanceID), nil)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
