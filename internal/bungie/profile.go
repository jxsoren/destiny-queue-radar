package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProfileComponents: 100 Profiles, 200 Characters, 204 CharacterActivities, 1000 Transitory.
const ProfileComponents = "100,200,204,1000"

// Privacy is Bungie's ComponentPrivacySetting.
type Privacy int

const (
	PrivacyNone    Privacy = 0
	PrivacyPublic  Privacy = 1
	PrivacyPrivate Privacy = 2
)

type single[T any] struct {
	Data    *T      `json:"data"`
	Privacy Privacy `json:"privacy"`
}

type perCharacter[T any] struct {
	Data    map[string]T `json:"data"`
	Privacy Privacy      `json:"privacy"`
}

type UserInfo struct {
	MembershipType              int64  `json:"membershipType"`
	MembershipID                string `json:"membershipId"`
	BungieGlobalDisplayName     string `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode int    `json:"bungieGlobalDisplayNameCode"`
}

type ProfileData struct {
	UserInfo       UserInfo  `json:"userInfo"`
	DateLastPlayed time.Time `json:"dateLastPlayed"`
	CharacterIDs   []string  `json:"characterIds"`
}

type Character struct {
	CharacterID          string    `json:"characterId"`
	DateLastPlayed       time.Time `json:"dateLastPlayed"`
	EmblemPath           string    `json:"emblemPath"`
	EmblemBackgroundPath string    `json:"emblemBackgroundPath"`
	ClassType            int       `json:"classType"`
}

type CharacterActivities struct {
	DateActivityStarted         time.Time `json:"dateActivityStarted"`
	CurrentActivityHash         uint32    `json:"currentActivityHash"`
	CurrentActivityModeHash     uint32    `json:"currentActivityModeHash"`
	CurrentActivityModeType     int       `json:"currentActivityModeType"`
	CurrentActivityModeHashes   []uint32  `json:"currentActivityModeHashes"`
	CurrentActivityModeTypes    []int     `json:"currentActivityModeTypes"`
	CurrentPlaylistActivityHash uint32    `json:"currentPlaylistActivityHash"`
}

type PartyMember struct {
	MembershipID string `json:"membershipId"`
	DisplayName  string `json:"displayName"`
	Status       int    `json:"status"`
}

type TransitoryActivity struct {
	StartTime         *time.Time `json:"startTime"`
	EndTime           *time.Time `json:"endTime"`
	NumberOfPlayers   int        `json:"numberOfPlayers"`
	NumberOfOpponents int        `json:"numberOfOpponents"`
}

type TransitoryData struct {
	PartyMembers    []PartyMember       `json:"partyMembers"`
	CurrentActivity *TransitoryActivity `json:"currentActivity"`
}

type Profile struct {
	Profile               single[ProfileData]               `json:"profile"`
	Characters            perCharacter[Character]           `json:"characters"`
	CharacterActivities   perCharacter[CharacterActivities] `json:"characterActivities"`
	ProfileTransitoryData single[TransitoryData]            `json:"profileTransitoryData"`

	// Raw is the undecoded Response JSON, kept for recording test data.
	Raw json.RawMessage `json:"-"`
}

func (c *Client) GetProfile(ctx context.Context, membershipType int64, membershipID string) (*Profile, error) {
	path := fmt.Sprintf("/Platform/Destiny2/%d/Profile/%s/?components=%s", membershipType, membershipID, ProfileComponents)
	raw, err := c.callRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("bungie: decode profile: %w", err)
	}
	p.Raw = raw
	return &p, nil
}

// ActiveCharacterID returns the character whose current activity started most recently.
func (p *Profile) ActiveCharacterID() (string, bool) {
	var bestID string
	var best time.Time
	for id, a := range p.CharacterActivities.Data {
		if bestID == "" || a.DateActivityStarted.After(best) {
			bestID, best = id, a.DateActivityStarted
		}
	}
	return bestID, bestID != ""
}
