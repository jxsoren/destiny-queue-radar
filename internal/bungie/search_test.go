package bungie

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestParseBungieName(t *testing.T) {
	tests := []struct {
		in       string
		name     string
		code     int
		wantFail bool
	}{
		{in: "Kwivor#1993", name: "Kwivor", code: 1993},
		{in: "Clae™#0880", name: "Clae™", code: 880},
		{in: "Badluck Apolo#7173", name: "Badluck Apolo", code: 7173},
		{in: "  Spaces#0001  ", name: "Spaces", code: 1},
		{in: "Has#Hash#1234", name: "Has#Hash", code: 1234},
		{in: "NoCode", wantFail: true},
		{in: "#1234", wantFail: true},
		{in: "Short#123", wantFail: true},
		{in: "Letters#12a4", wantFail: true},
	}
	for _, tt := range tests {
		name, code, err := ParseBungieName(tt.in)
		if tt.wantFail {
			if err == nil {
				t.Errorf("%q: expected error", tt.in)
			}
			continue
		}
		if err != nil || name != tt.name || code != tt.code {
			t.Errorf("%q: got (%q, %d, %v), want (%q, %d)", tt.in, name, code, err, tt.name, tt.code)
		}
	}
}

func TestFormatBungieName_PadsCode(t *testing.T) {
	if got := FormatBungieName("Clae™", 880); got != "Clae™#0880" {
		t.Errorf("got %q", got)
	}
}

func TestSearchByBungieName_PicksCrossSavePrimary(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/Platform/Destiny2/SearchDestinyPlayerByBungieName/-1/" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["displayName"] != "Kwivor" || body["displayNameCode"] != float64(1993) {
			t.Errorf("body = %v", body)
		}
		w.Write([]byte(`{"ErrorCode":1,"Response":[
			{"membershipType":2,"membershipId":"psn","crossSaveOverride":3,"bungieGlobalDisplayName":"Kwivor","bungieGlobalDisplayNameCode":1993},
			{"membershipType":3,"membershipId":"steam","crossSaveOverride":3,"bungieGlobalDisplayName":"Kwivor","bungieGlobalDisplayNameCode":1993}
		]}`))
	})
	card, err := c.SearchByBungieName(context.Background(), "Kwivor", 1993)
	if err != nil {
		t.Fatal(err)
	}
	if card.MembershipID != "steam" || card.MembershipType != 3 {
		t.Errorf("card = %+v", card)
	}
}

func TestSearchByBungieName_NoCrossSaveUsesFirst(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":[{"membershipType":3,"membershipId":"only","crossSaveOverride":0}]}`))
	})
	card, err := c.SearchByBungieName(context.Background(), "X", 1)
	if err != nil || card.MembershipID != "only" {
		t.Errorf("card=%+v err=%v", card, err)
	}
}

func TestSearchByBungieName_EmptyIsNotFound(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ErrorCode":1,"Response":[]}`))
	})
	_, err := c.SearchByBungieName(context.Background(), "Nobody", 1)
	if !IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}
