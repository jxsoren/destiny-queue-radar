package bungie

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestActivityHistory_RequestsPvPModeAndDecodes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Platform/Destiny2/3/Account/4611/Character/c1/Stats/Activities/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if q := r.URL.Query(); q.Get("mode") != "5" || q.Get("count") != "10" {
			t.Errorf("query = %v", q)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"ErrorCode":1,"Response":{"activities":[
			{"period":"2026-10-15T18:00:00Z","activityDetails":{"referenceId":100,"directorActivityHash":200,"instanceId":"9001","mode":84,"modes":[5,84]}}
		]}}`))
	})
	acts, err := c.ActivityHistory(context.Background(), "tok", 3, "4611", "c1", ModeAllPvP, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	if len(acts) != 1 || !acts[0].Period.Equal(want) || acts[0].ActivityDetails.InstanceID != "9001" ||
		acts[0].ActivityDetails.ReferenceID != 100 || acts[0].ActivityDetails.DirectorActivityHash != 200 {
		t.Errorf("acts = %+v", acts)
	}
}

func TestActivityHistory_EmptyResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{}}`))
	})
	acts, err := c.ActivityHistory(context.Background(), "", 3, "4611", "c1", ModeAllPvP, 10)
	if err != nil || len(acts) != 0 {
		t.Errorf("acts=%v err=%v", acts, err)
	}
}

func TestGetPGCR_DecodesEntries(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Platform/Destiny2/Stats/PostGameCarnageReport/9001/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"ErrorCode":1,"Response":{
			"period":"2026-10-15T18:00:00Z",
			"activityDetails":{"referenceId":100,"directorActivityHash":200,"instanceId":"9001","mode":84},
			"entries":[
				{"characterId":"c1","player":{"destinyUserInfo":{"membershipType":3,"membershipId":"p1","bungieGlobalDisplayName":"Ghost","bungieGlobalDisplayNameCode":880,"iconPath":"/e.jpg"}},
				 "values":{"team":{"basic":{"value":17,"displayValue":"Alpha"}}}},
				{"characterId":"c9","player":{"destinyUserInfo":{"membershipType":3,"membershipId":"p2"}},"values":{}}
			]}}`))
	})
	r, err := c.GetPGCR(context.Background(), "9001")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Entries) != 2 || r.ActivityDetails.ReferenceID != 100 {
		t.Fatalf("report = %+v", r)
	}
	e := r.Entries[0]
	u := e.Player.DestinyUserInfo
	if u.MembershipID != "p1" || u.BungieGlobalDisplayName != "Ghost" || u.IconPath != "/e.jpg" || e.Team() != 17 {
		t.Errorf("entry = %+v team=%d", e, e.Team())
	}
	if r.Entries[1].Team() != -1 {
		t.Errorf("missing team = %d, want -1", r.Entries[1].Team())
	}
}

func TestActivityHistory_DecodesDurationAndEndedAt(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"activities":[
			{"period":"2026-10-15T18:00:00Z","activityDetails":{"directorActivityHash":200,"instanceId":"9001","mode":84},
			 "values":{"activityDurationSeconds":{"statId":"activityDurationSeconds","basic":{"value":612.0,"displayValue":"10m 12s"}},
			           "completed":{"basic":{"value":1.0,"displayValue":"Yes"}}}},
			{"period":"2026-10-15T17:00:00Z","activityDetails":{"instanceId":"9000","mode":84}}
		]}}`))
	})
	acts, err := c.ActivityHistory(context.Background(), "", 3, "4611", "c1", ModeAllPvP, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 2 {
		t.Fatalf("acts = %+v", acts)
	}
	start := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	if got := acts[0].Duration(); got != 612*time.Second {
		t.Errorf("duration = %s, want 10m12s", got)
	}
	if got := acts[0].EndedAt(); !got.Equal(start.Add(612 * time.Second)) {
		t.Errorf("EndedAt = %s, want %s", got, start.Add(612*time.Second))
	}
	// Without a duration the start is the best guess for the end.
	if got := acts[1].EndedAt(); !got.Equal(acts[1].Period) || acts[1].Duration() != 0 {
		t.Errorf("EndedAt without duration = %s (duration %s), want the period", got, acts[1].Duration())
	}
}
