package web

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

func addBody(id string) map[string]any {
	return map[string]any{"membershipType": 3, "membershipId": id, "bungieName": "Player#0001", "note": "aimbot", "source": "search"}
}

func TestWatchlist_AddListUpdateRemove(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")

	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	e.store.watch[u.ID][0].Status = status.Status{State: status.InMatch, Activity: "Burnout"}

	rec := e.do(t, http.MethodGet, "/api/watchlist", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d", rec.Code)
	}
	list := decode[[]playerJSON](t, rec)
	if len(list) != 1 || list[0].MembershipID != "4611" || list[0].Note != "aimbot" || list[0].Status.State != status.InMatch || list[0].BungieName != "Player#0001" {
		t.Errorf("list = %+v", list)
	}

	if rec := e.do(t, http.MethodPatch, "/api/watchlist/4611", map[string]any{"note": "wallhacks"}, cookie); rec.Code != http.StatusNoContent {
		t.Errorf("patch = %d %s", rec.Code, rec.Body)
	}
	if e.store.watch[u.ID][0].Note != "wallhacks" {
		t.Error("note not updated")
	}
	if rec := e.do(t, http.MethodPatch, "/api/watchlist/999", map[string]any{"note": "x"}, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("patch unknown = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/4611", nil, cookie); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/4611", nil, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("delete again = %d", rec.Code)
	}
}

func TestWatchlist_EmptyListIsArray(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	rec := e.do(t, http.MethodGet, "/api/watchlist", nil, cookie)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("body = %q, want []", rec.Body.String())
	}
}

func TestWatchlist_AddValidation(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	bad := []map[string]any{
		{"membershipType": 3, "membershipId": "", "source": "search"},
		{"membershipType": 3, "membershipId": "12ab", "source": "search"},
		{"membershipType": 0, "membershipId": "4611", "source": "search"},
		{"membershipType": 3, "membershipId": "4611", "source": "rumor"},
		{"membershipType": 3, "membershipId": "4611", "source": "search", "note": strings.Repeat("x", maxNoteLen+1)},
		{"membershipType": 3, "membershipId": "4611", "source": "search", "extra": true},
	}
	for i, body := range bad {
		if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusBadRequest {
			t.Errorf("case %d: code = %d, want 400", i, rec.Code)
		}
	}
	if rec := e.do(t, http.MethodPatch, "/api/watchlist/4611", map[string]any{"note": strings.Repeat("x", maxNoteLen+1)}, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("long note patch = %d", rec.Code)
	}
}

func TestWatchlist_FullReturnsConflict(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.store.addErr = store.ErrWatchlistFull
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusConflict {
		t.Errorf("code = %d, want 409", rec.Code)
	}
}

func TestSearch(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost™#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777", BungieGlobalDisplayName: "Ghost™", BungieGlobalDisplayNameCode: 880}

	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%E2%84%A2%230880", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	got := decode[searchResultJSON](t, rec)
	if got.MembershipID != "777" || got.MembershipType != 3 || got.BungieName != "Ghost™#0880" {
		t.Errorf("result = %+v", got)
	}
	if rec := e.do(t, http.MethodGet, "/api/search?q=NoCode", nil, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("bad name = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodGet, "/api/search?q=Nobody%230001", nil, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("unknown = %d", rec.Code)
	}
	e.bungie.searchErr = errors.New("bungie down")
	if rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%E2%84%A2%230880", nil, cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("bungie error = %d", rec.Code)
	}
}

func TestSearch_RateLimited(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	for i := 0; i < lookupsPerMinute; i++ {
		e.do(t, http.MethodGet, "/api/search?q=Nobody%230001", nil, cookie)
	}
	rec := e.do(t, http.MethodGet, "/api/search?q=Nobody%230001", nil, cookie)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "60" {
		t.Errorf("code = %d Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}
