package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

func addBody(id string) map[string]any {
	return map[string]any{"membershipType": 3, "membershipId": id, "bungieName": "Player#0001", "source": "search"}
}

func TestWatchlist_AddListRemove(t *testing.T) {
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
	if len(list) != 1 || list[0].MembershipID != "4611" || list[0].Status.State != status.InMatch || list[0].BungieName != "Player#0001" {
		t.Errorf("list = %+v", list)
	}
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/4611", nil, cookie); rec.Code != http.StatusNoContent {
		t.Errorf("delete = %d", rec.Code)
	}
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/4611", nil, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("delete again = %d", rec.Code)
	}
}

func TestWatchlist_HasNoNotes(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Player", 1)
	withNote := addBody("4611")
	withNote["note"] = "aimbot"
	if rec := e.do(t, http.MethodPost, "/api/watchlist", withNote, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("add with note = %d, want 400 (unknown field)", rec.Code)
	}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	rec := e.do(t, http.MethodGet, "/api/watchlist", nil, cookie)
	if strings.Contains(rec.Body.String(), `"note"`) {
		t.Errorf("watchlist JSON still has a note: %s", rec.Body)
	}
	if rec := e.do(t, http.MethodPatch, "/api/watchlist/4611", map[string]any{"note": "x"}, cookie); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("patch = %d, want 405", rec.Code)
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
		{"membershipType": 3, "membershipId": "4611", "source": "search", "extra": true},
		{"membershipType": 999, "membershipId": "4611", "source": "search"},
		{"membershipType": 10, "membershipId": "4611", "source": "search"},
		{"membershipType": 254, "membershipId": "4611", "source": "search"},
		{"membershipType": 3, "membershipId": strings.Repeat("1", 20), "source": "search"},
	}
	for i, body := range bad {
		if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusBadRequest {
			t.Errorf("case %d: code = %d, want 400", i, rec.Code)
		}
	}
}

func TestWatchlist_PathIDsValidated(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	if rec := e.do(t, http.MethodDelete, "/api/watchlist/abc", nil, cookie); rec.Code != http.StatusBadRequest {
		t.Errorf("delete bad id = %d", rec.Code)
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
	e.bungie.profileErr = &bungie.APIError{Code: bungie.CodeSystemDisabled, Status: "SystemDisabled"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("maintenance = %d, want 503", rec.Code)
	}
	e.bungie.profileErr = &bungie.APIError{Code: 36, Status: "ThrottleLimitExceededMinutes", ThrottleSeconds: 30}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("throttled = %d, want 503", rec.Code)
	}
	e.bungie.profileErr = &bungie.APIError{Code: 7, Status: "ParameterParseFailure"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("other api error = %d, want 502", rec.Code)
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

func TestWatchlist_AddRejectsMismatchedProfile(t *testing.T) {
	e := newTestEnv(t)
	u, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "9999", "Other", 1) // Bungie answers for a different ID than asked
	e.bungie.profiles["4611"] = e.bungie.profiles["9999"]
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("mismatched id = %d, want 502", rec.Code)
	}
	e.addProfile(254, "4611", "Odd", 1) // a type outside the allowlist
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusBadGateway {
		t.Errorf("invalid type in response = %d, want 502", rec.Code)
	}
	if len(e.store.watch[u.ID]) != 0 {
		t.Error("player stored from an unexpected response")
	}
}

func TestSearch_IncludesEmblem(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777", BungieGlobalDisplayName: "Ghost", BungieGlobalDisplayNameCode: 880}
	e.addProfile(3, "777", "Ghost", 880)
	e.bungie.profiles["777"].Characters.Data = map[string]bungie.Character{
		"c1": {CharacterID: "c1", EmblemPath: "/common/emblem.jpg", EmblemBackgroundPath: "/common/emblem-bg.jpg"},
	}

	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	got := decode[searchResultJSON](t, rec)
	if got.EmblemPath != "/common/emblem.jpg" || got.EmblemBackgroundPath != "/common/emblem-bg.jpg" || got.MembershipID != "777" {
		t.Errorf("result = %+v", got)
	}
}

func TestSearch_ProfileErrorStillAnswers(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777", BungieGlobalDisplayName: "Ghost", BungieGlobalDisplayNameCode: 880}
	e.bungie.profileErr = errors.New("bungie down")

	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	got := decode[searchResultJSON](t, rec)
	if got.EmblemPath != "" || got.BungieName != "Ghost#0880" {
		t.Errorf("result = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"emblemPath":""`) {
		t.Errorf("emblemPath must always be present: %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"emblemBackgroundPath":""`) {
		t.Errorf("emblemBackgroundPath must always be present: %s", rec.Body)
	}
}

func TestSearch_EmblemLookupUsesNoExtraLookupBudget(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777"}
	for i := 0; i < lookupsPerMinute; i++ {
		if rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie); rec.Code != http.StatusOK {
			t.Fatalf("search %d = %d, want 200 (one lookup per search)", i, rec.Code)
		}
	}
	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("search 31 = %d retry-after %q, want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// hangingProfiles blocks GetProfile until its context ends.
type hangingProfiles struct{ *fakeBungie }

func (h hangingProfiles) GetProfile(ctx context.Context, _ int64, _ string) (*bungie.Profile, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSearch_SlowProfileLookupIsBounded(t *testing.T) {
	old := searchEmblemTimeout
	searchEmblemTimeout = 20 * time.Millisecond
	defer func() { searchEmblemTimeout = old }()
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777", BungieGlobalDisplayName: "Ghost", BungieGlobalDisplayNameCode: 880}
	e.s.d.Bungie = hangingProfiles{e.bungie}

	start := time.Now()
	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie)
	if took := time.Since(start); took > time.Second {
		t.Errorf("search took %v, want it bounded by the emblem timeout", took)
	}
	if rec.Code != http.StatusOK || decode[searchResultJSON](t, rec).EmblemPath != "" {
		t.Errorf("search = %d %s", rec.Code, rec.Body)
	}
}

func TestSearch_ProfileWithoutCharactersHasNoEmblem(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.bungie.cards["Ghost#0880"] = bungie.UserInfoCard{MembershipType: 3, MembershipID: "777", BungieGlobalDisplayName: "Ghost", BungieGlobalDisplayNameCode: 880}
	e.addProfile(3, "777", "Ghost", 880)
	e.bungie.profiles["777"].CharacterActivities.Data = map[string]bungie.CharacterActivities{"c1": {}}

	rec := e.do(t, http.MethodGet, "/api/search?q=Ghost%230880", nil, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emblemPath":""`) {
		t.Errorf("search = %d %s", rec.Code, rec.Body)
	}
}

func TestWatchlist_AddRechecksPlayerWithBungieIdentity(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	e.addProfile(3, "4611", "Real", 42)
	body := map[string]any{"membershipType": 2, "membershipId": "4611", "source": "search"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", body, cookie); rec.Code != http.StatusCreated {
		t.Fatalf("add = %d %s", rec.Code, rec.Body)
	}
	got := e.engine.rechecked
	if len(got) != 1 || got[0].MembershipType != 3 || got[0].MembershipID != "4611" {
		t.Errorf("rechecked = %+v, want [{3 4611}]", got)
	}
}

func TestWatchlist_NoRecheckWhenAddFails(t *testing.T) {
	e := newTestEnv(t)
	_, cookie := e.login(t, "Guardian#1993")
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown player = %d, want 404", rec.Code)
	}
	e.bungie.profileErr = &bungie.APIError{Code: bungie.CodeSystemDisabled, Status: "SystemDisabled"}
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("maintenance = %d, want 503", rec.Code)
	}
	e.bungie.profileErr = nil
	e.addProfile(3, "4611", "Real", 42)
	e.store.addErr = store.ErrWatchlistFull
	if rec := e.do(t, http.MethodPost, "/api/watchlist", addBody("4611"), cookie); rec.Code != http.StatusConflict {
		t.Fatalf("full = %d, want 409", rec.Code)
	}
	if n := len(e.engine.rechecked); n != 0 {
		t.Errorf("rechecked %d times on failed adds, want 0", n)
	}
}
