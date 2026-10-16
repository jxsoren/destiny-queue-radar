package bungie

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

const membershipsBody = `{"ErrorCode":1,"Response":{
	"bungieNetUser":{"membershipId":"bnet-1","uniqueName":"Fallback#0001"},
	"primaryMembershipId":"steam",
	"destinyMemberships":[
		{"membershipType":2,"membershipId":"psn","crossSaveOverride":3,"bungieGlobalDisplayName":"Guardian","bungieGlobalDisplayNameCode":1993},
		{"membershipType":3,"membershipId":"steam","crossSaveOverride":3,"bungieGlobalDisplayName":"Guardian","bungieGlobalDisplayNameCode":1993}
	]}}`

func TestGetCurrentUser_SendsBearerAndPrefersPrimaryMembership(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Platform/User/GetMembershipsForCurrentUser/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Error("missing X-API-Key")
		}
		w.Write([]byte(membershipsBody))
	})
	u, err := c.GetCurrentUser(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.BungieNetMembershipID != "bnet-1" || u.BungieName != "Guardian#1993" {
		t.Errorf("user = %+v", u)
	}
	if u.Destiny.MembershipID != "steam" || u.Destiny.MembershipType != 3 {
		t.Errorf("destiny = %+v, want primaryMembershipId steam", u.Destiny)
	}
}

func TestGetCurrentUser_FallsBackToCrossSaveAndUniqueName(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{
			"bungieNetUser":{"membershipId":"bnet-2","uniqueName":"Only#0420"},
			"destinyMemberships":[
				{"membershipType":1,"membershipId":"xbox","crossSaveOverride":3},
				{"membershipType":3,"membershipId":"steam","crossSaveOverride":3}
			]}}`))
	})
	u, err := c.GetCurrentUser(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.Destiny.MembershipID != "steam" || u.BungieName != "Only#0420" {
		t.Errorf("user = %+v", u)
	}
}

func TestGetCurrentUser_NoDestinyAccount(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{"bungieNetUser":{"membershipId":"x"},"destinyMemberships":[]}}`))
	})
	if _, err := c.GetCurrentUser(context.Background(), "tok"); !errors.Is(err, ErrNoDestinyAccount) {
		t.Errorf("err = %v", err)
	}
}

func TestCall_SendsNoAuthorizationWithoutToken(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("unauthenticated call sent Authorization")
		}
		w.Write([]byte(`{"Response":{"value":"ok"},"ErrorCode":1}`))
	})
	if _, err := call[echo](context.Background(), c, http.MethodGet, "/Platform/Test/", nil); err != nil {
		t.Fatal(err)
	}
}

// Pins precedence only: primaryMembershipId wins even when Cross Save ordering points elsewhere.
// Real data keeps these consistent.
func TestGetCurrentUser_PrimaryMembershipIDWinsOverCrossSave(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{
			"bungieNetUser":{"membershipId":"b"},
			"primaryMembershipId":"steam",
			"destinyMemberships":[
				{"membershipType":2,"membershipId":"psn","crossSaveOverride":2},
				{"membershipType":3,"membershipId":"steam"}
			]}}`))
	})
	u, err := c.GetCurrentUser(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.Destiny.MembershipID != "steam" {
		t.Errorf("destiny = %+v, want steam", u.Destiny)
	}
}

func TestGetCurrentUser_NameFallsBackToCachedGlobalName(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{
			"bungieNetUser":{"membershipId":"b","uniqueName":"Legacy","cachedBungieGlobalDisplayName":"Cached","cachedBungieGlobalDisplayNameCode":42},
			"destinyMemberships":[{"membershipType":3,"membershipId":"steam"}]}}`))
	})
	u, err := c.GetCurrentUser(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.BungieName != "Cached#0042" {
		t.Errorf("name = %q", u.BungieName)
	}
}

func TestGetCurrentUser_NameEmptyWhenNoSources(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":{
			"bungieNetUser":{"membershipId":"b"},
			"destinyMemberships":[{"membershipType":3,"membershipId":"steam"}]}}`))
	})
	u, err := c.GetCurrentUser(context.Background(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if u.BungieName != "" {
		t.Errorf("name = %q, want empty", u.BungieName)
	}
}
