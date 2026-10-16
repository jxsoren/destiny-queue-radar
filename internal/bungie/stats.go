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
	Period          time.Time       `json:"period"`
	ActivityDetails ActivityDetails `json:"activityDetails"`
}

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
