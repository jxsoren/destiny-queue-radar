package bungie

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const profileFixture = `{
  "ErrorCode": 1,
  "Response": {
    "profile": {"privacy": 1, "data": {
      "userInfo": {"membershipType": 3, "membershipId": "4611", "bungieGlobalDisplayName": "Kwivor", "bungieGlobalDisplayNameCode": 1993},
      "dateLastPlayed": "2026-10-15T18:00:00Z",
      "characterIds": ["c1", "c2"]
    }},
    "characters": {"privacy": 1, "data": {
      "c1": {"characterId": "c1", "dateLastPlayed": "2026-10-15T18:00:00Z", "emblemPath": "/e1.jpg", "emblemBackgroundPath": "/e1bg.jpg", "classType": 1},
      "c2": {"characterId": "c2", "dateLastPlayed": "2026-10-01T10:00:00Z", "emblemPath": "/e2.jpg", "emblemBackgroundPath": "/e2bg.jpg", "classType": 2}
    }},
    "characterActivities": {"privacy": 1, "data": {
      "c1": {"dateActivityStarted": "2026-10-15T18:05:00Z", "currentActivityHash": 111, "currentActivityModeHash": 222, "currentActivityModeType": 69, "currentPlaylistActivityHash": 333},
      "c2": {"dateActivityStarted": "2026-10-01T10:00:00Z", "currentActivityHash": 0}
    }},
    "profileTransitoryData": {"privacy": 1, "data": {
      "partyMembers": [{"membershipId": "4611", "displayName": "Kwivor", "status": 11}],
      "currentActivity": {"startTime": "2026-10-15T18:05:10Z", "numberOfPlayers": 6, "numberOfOpponents": 3}
    }}
  }
}`

func TestGetProfile_RequestsComponentsAndDecodes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/Platform/Destiny2/3/Profile/4611/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("components"); got != ProfileComponents {
			t.Errorf("components = %q", got)
		}
		w.Write([]byte(profileFixture))
	})
	p, err := c.GetProfile(context.Background(), 3, "4611")
	if err != nil {
		t.Fatal(err)
	}
	if p.Profile.Data.UserInfo.BungieGlobalDisplayName != "Kwivor" {
		t.Errorf("display name = %q", p.Profile.Data.UserInfo.BungieGlobalDisplayName)
	}
	if p.Characters.Data["c1"].EmblemBackgroundPath != "/e1bg.jpg" {
		t.Errorf("emblem bg = %q", p.Characters.Data["c1"].EmblemBackgroundPath)
	}
	if a := p.CharacterActivities.Data["c1"]; a.CurrentActivityModeHash != 222 || a.CurrentPlaylistActivityHash != 333 {
		t.Errorf("activities = %+v", a)
	}
	if p.ProfileTransitoryData.Data == nil || len(p.ProfileTransitoryData.Data.PartyMembers) != 1 {
		t.Errorf("transitory = %+v", p.ProfileTransitoryData.Data)
	}
	if p.CharacterActivities.Privacy != PrivacyPublic {
		t.Errorf("privacy = %d", p.CharacterActivities.Privacy)
	}
	if len(p.Raw) == 0 || !strings.Contains(string(p.Raw), "profileTransitoryData") {
		t.Errorf("Raw not populated")
	}
}

func TestGetProfile_OfflineHasNoTransitoryData(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"profileTransitoryData":{"privacy":1}}}`))
	})
	p, err := c.GetProfile(context.Background(), 3, "4611")
	if err != nil {
		t.Fatal(err)
	}
	if p.ProfileTransitoryData.Data != nil {
		t.Errorf("expected nil transitory data for offline player")
	}
}

func TestActiveCharacterID_PicksMostRecentActivity(t *testing.T) {
	p := &Profile{}
	p.CharacterActivities.Data = map[string]CharacterActivities{
		"old": {DateActivityStarted: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		"new": {DateActivityStarted: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)},
	}
	id, ok := p.ActiveCharacterID()
	if !ok || id != "new" {
		t.Errorf("got %q, %v", id, ok)
	}
}

func TestActiveCharacterID_NoData(t *testing.T) {
	if _, ok := (&Profile{}).ActiveCharacterID(); ok {
		t.Error("expected ok=false with no activity data")
	}
}
