package web

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

func lobby(id string, self string) matches.Match {
	return matches.Match{InstanceID: id, Activity: "Burnout", Playlist: "Trials of Osiris", Players: []matches.Participant{
		{MembershipType: 3, MembershipID: self, BungieName: "Guardian#1993", Team: 17},
		{MembershipType: 3, MembershipID: "777", BungieName: "Ghost#0880", Team: 18},
		{MembershipType: 3, MembershipID: "888", BungieName: "Other#0001", Team: 18},
	}}
}

func TestRecentMatches_MarksSelfAndWatched(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.tokens.access = "acc"
	e.matches.recent = []matches.Match{lobby("9001", u.DestinyMembershipID)}
	e.store.AddWatch(context.Background(), u.ID, store.PlayerRef{MembershipType: 3, MembershipID: "777"}, "", store.SourceMatch)

	rec := e.do(t, http.MethodGet, "/api/matches/recent", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d %s", rec.Code, rec.Body)
	}
	got := decode[[]matchJSON](t, rec)
	if len(got) != 1 || len(got[0].Players) != 3 || got[0].Playlist != "Trials of Osiris" {
		t.Fatalf("matches = %+v", got)
	}
	p := got[0].Players
	if !p[0].IsSelf || p[0].Watched || p[1].IsSelf || !p[1].Watched || p[2].Watched {
		t.Errorf("flags = %+v", p)
	}
	if e.matches.gotToken != "acc" || e.matches.gotMember != u.DestinyMembershipID {
		t.Errorf("token=%q member=%q", e.matches.gotToken, e.matches.gotMember)
	}
}

func TestRecentMatches_Errors(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")

	e.tokens.accessErr = auth.ErrReauthRequired
	rec := e.do(t, http.MethodGet, "/api/matches/recent", nil, cookie)
	if rec.Code != http.StatusUnauthorized || decode[map[string]any](t, rec)["reauth"] != true {
		t.Errorf("reauth = %d %s", rec.Code, rec.Body)
	}

	e.tokens.accessErr = nil
	e.matches.recentErr = errors.New("bungie down")
	if rec := e.do(t, http.MethodGet, "/api/matches/recent", nil, cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("bungie error = %d", rec.Code)
	}

	e.store.SetDestinyMembership(context.Background(), u.ID, 0, "")
	if rec := e.do(t, http.MethodGet, "/api/matches/recent", nil, cookie); rec.Code != http.StatusConflict {
		t.Errorf("no destiny account = %d", rec.Code)
	}
}

func TestMatchByID(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.matches.byID["9001"] = lobby("9001", u.DestinyMembershipID)

	rec := e.do(t, http.MethodGet, "/api/matches/9001", nil, cookie)
	if rec.Code != http.StatusOK || decode[matchJSON](t, rec).InstanceID != "9001" {
		t.Errorf("match = %d %s", rec.Code, rec.Body)
	}
	if rec := e.do(t, http.MethodGet, "/api/matches/abc", nil, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("non-numeric id = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodGet, "/api/matches/404", nil, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id = %d", rec.Code)
	}
}
