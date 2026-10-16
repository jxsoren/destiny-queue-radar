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
	e.addProfile(3, "4611", "Player", 1)

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
		{"membershipType": 999, "membershipId": "4611", "source": "search"},
		{"membershipType": 3, "membershipId": strings.Repeat("1", 20), "source": "search"},
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

func TestWatchlist_NoteLimitCountsRunes(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Player", 1)
	body := addBody("4611")
	body["note"] = strings.Repeat("é", maxNoteLen)
	if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusCreated {
		t.Errorf("%d multibyte runes = %d, want 201", maxNoteLen, rec.Code)
	}
	body["note"] = strings.Repeat("é", maxNoteLen+1)
	if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("%d multibyte runes = %d, want 400", maxNoteLen+1, rec.Code)
	}
}

func TestWatchlist_PathIDsValidated(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	if rec := e.do(t, http.MethodPatch, "/api/watchlist/abc", map[string]any{"note": "x"}, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("patch bad id = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/"+strings.Repeat("1", 20), nil, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("delete long id = %d", rec.Code)
	}
}

func TestWatchlist_AddUsesBungieIdentityNotClients(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Real", 42)
	body := map[string]any{"membershipType": 2, "membershipId": "4611", "bungieName": "Evil<script>", "source": "search"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	got := e.store.watch[u.ID][0]
	if got.MembershipType != 3 || got.BungieName != "Real#0042" {
		t.Errorf("stored type=%d name=%q, want Bungie's 3 / Real#0042", got.MembershipType, got.BungieName)
	}
}

func TestWatchlist_AddWithoutProfileDataUsesRequestIDAndNoName(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.bungie.profiles["4611"] = &bungie.Profile{} // private profile component
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	got := e.store.watch[u.ID][0]
	if got.MembershipType != 3 || got.MembershipID != "4611" || got.BungieName != "" {
		t.Errorf("stored = %+v", got.PlayerRef)
	}
}

func TestWatchlist_AddVerificationFailures(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusNotFound {
		t.Errorf("unknown player = %d, want 404", rec.Code)
	}
	e.bungie.profileErr = &bungie.APIError{Code: 1601, Status: "DestinyAccountNotFound"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusNotFound {
		t.Errorf("api error code = %d, want 404", rec.Code)
	}
	e.bungie.profileErr = errors.New("bungie down")
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("bungie error = %d, want 502", rec.Code)
	}
	if len(e.store.watch[u.ID]) != 0 {
		t.Error("player stored despite failed verification")
	}
}

func TestWatchlist_AddIsRateLimited(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Player", 1)
	for i := 0; i < lookupsPerMinute; i++ {
		e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie)
	}
	rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("code = %d Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestWatchlist_FullReturnsConflict(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Player", 1)
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
