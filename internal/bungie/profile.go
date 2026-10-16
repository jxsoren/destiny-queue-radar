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

// single is a component with one data payload. Data is nil when the component is
// private (Privacy 2), not returned, or (for transitory data) the player is offline.
// Privacy 0 means "not returned", not public.
type single[T any] struct {
	Data    *T      `json:"data"`
	Privacy Privacy `json:"privacy"`
}

// perCharacter is a component keyed by character ID. Data is nil/empty when the
// component is private (Privacy 2) or not returned; Privacy 0 means "not returned", not public.
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

// Profile is the decoded profile response. For every component, Data is nil (or an
// empty map) when it is private (Privacy 2) or not returned; Privacy 0 means
// "not returned", not public. ProfileTransitoryData.Data is also nil when the player is offline.
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

// ActiveCharacterID returns the character most likely in play. Ordering: later
// current-activity start wins; ties go to the later character DateLastPlayed;
// remaining ties go to the lexicographically smaller ID, so the result is deterministic.
func (p *Profile) ActiveCharacterID() (string, bool) {
	var best string
	for id := range p.CharacterActivities.Data {
		if best == "" || p.moreActive(id, best) {
			best = id
		}
	}
	return best, best != ""
}

// moreActive reports whether character a should be preferred over b.
func (p *Profile) moreActive(a, b string) bool {
	sa, sb := p.CharacterActivities.Data[a].DateActivityStarted, p.CharacterActivities.Data[b].DateActivityStarted
	if !sa.Equal(sb) {
		return sa.After(sb)
	}
	la, lb := p.Characters.Data[a].DateLastPlayed, p.Characters.Data[b].DateLastPlayed
	if !la.Equal(lb) {
		return la.After(lb)
	}
	return a < b
}

// ActiveEmblem returns the emblem icon and banner of the character in use: the active
// character when there is activity data, else the most recently played one. It returns empty
// paths for a nil profile or one without characters.
func (p *Profile) ActiveEmblem() (icon, background string) {
	if p == nil {
		return "", ""
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		var latest time.Time
		for cid, c := range p.Characters.Data {
			if id == "" || c.DateLastPlayed.After(latest) || (c.DateLastPlayed.Equal(latest) && cid < id) {
				id, latest = cid, c.DateLastPlayed
			}
		}
	}
	c := p.Characters.Data[id]
	return c.EmblemPath, c.EmblemBackgroundPath
}
