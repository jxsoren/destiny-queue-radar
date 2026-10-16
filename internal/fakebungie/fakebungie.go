// Package fakebungie is a scriptable stand-in for the Bungie.net API (main spec §8.3). It serves
// the manifest, profiles, Bungie Name search, PvP history and post-game reports for a fixed
// roster; POST /fake/players/{id}/state changes what a player is doing. It is for end-to-end
// tests and running the dashboard without an API key, never for production.
package fakebungie

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

type Server struct {
	mu      sync.Mutex
	now     func() time.Time
	players map[string]*player
	lobbies map[string]lobby
	mux     *http.ServeMux
}

func New() *Server {
	s := &Server{now: time.Now, mux: http.NewServeMux()}
	t := s.now()
	s.players = roster(t)
	s.lobbies = lobbies(t)
	s.mux.HandleFunc("GET /Platform/Destiny2/Manifest/{$}", s.handleManifest)
	s.mux.HandleFunc("GET /fake/manifest/activities.json", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, activityDefs) })
	s.mux.HandleFunc("GET /fake/manifest/modes.json", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, modeDefs) })
	s.mux.HandleFunc("GET /Platform/Destiny2/{type}/Profile/{id}/{$}", s.handleProfile)
	s.mux.HandleFunc("POST /Platform/Destiny2/SearchDestinyPlayerByBungieName/{type}/{$}", s.handleSearch)
	s.mux.HandleFunc("GET /Platform/Destiny2/{type}/Account/{id}/Character/{cid}/Stats/Activities/{$}", s.handleHistory)
	s.mux.HandleFunc("GET /Platform/Destiny2/Stats/PostGameCarnageReport/{id}/{$}", s.handlePGCR)
	s.mux.HandleFunc("POST /fake/players/{id}/state", s.handleSetState)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

var errUnknownPlayer = errors.New("fakebungie: unknown player")

// SetState changes what a player is doing from now on.
func (s *Server) SetState(id string, st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.players[id]
	if !ok {
		return errUnknownPlayer
	}
	p.state, p.since = st, s.now()
	return nil
}

func (s *Server) state(id string) State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.players[id]; ok {
		return p.state
	}
	return ""
}

func (s *Server) handleManifest(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, map[string]any{
		"version": "fake-1",
		"jsonWorldComponentContentPaths": map[string]any{"en": map[string]string{
			"DestinyActivityDefinition":     "/fake/manifest/activities.json",
			"DestinyActivityModeDefinition": "/fake/manifest/modes.json",
		}},
	})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	p, ok := s.players[r.PathValue("id")]
	var body map[string]any
	if ok {
		body = profileJSON(p)
	}
	s.mu.Unlock()
	if !ok {
		writeBungieError(w, bungie.CodeAccountNotFound, "DestinyAccountNotFound")
		return
	}
	writeOK(w, body)
}

func profileJSON(p *player) map[string]any {
	profile := map[string]any{"privacy": 1, "data": map[string]any{
		"userInfo":       card(p),
		"dateLastPlayed": p.since,
		"characterIds":   []string{p.characterID()},
	}}
	cid := p.characterID()
	chars := map[string]any{"privacy": 1, "data": map[string]any{cid: map[string]any{
		"characterId": cid, "dateLastPlayed": p.since, "emblemPath": p.emblem(),
		"emblemBackgroundPath": strings.Replace(p.emblem(), ".jpg", "_bg.jpg", 1), "classType": 0,
	}}}
	if p.state == Private {
		// A private player hides what they are doing but not their characters or emblem.
		hidden := map[string]any{"privacy": 2}
		return map[string]any{"profile": profile, "characters": chars, "characterActivities": hidden, "profileTransitoryData": hidden}
	}
	activity := map[string]any{"dateActivityStarted": p.since, "currentActivityHash": 0}
	transitory := map[string]any{"privacy": 1, "data": map[string]any{"partyMembers": []any{}, "currentActivity": map[string]any{"startTime": p.since}}}
	switch p.state {
	case Orbit:
		activity["currentActivityHash"] = OrbitHash
	case InMatch:
		activity["currentActivityHash"] = BurnoutHash
		activity["currentActivityModeHash"] = CompetitiveModeHash
		activity["currentActivityModeHashes"] = []uint32{CompetitiveModeHash}
		activity["currentActivityModeType"] = 69
		activity["currentPlaylistActivityHash"] = CompetitiveHash
	case PvE:
		activity["currentActivityHash"] = StrikeHash
		activity["currentActivityModeHash"] = StrikeModeHash
		activity["currentActivityModeType"] = 18
	case Offline:
		transitory = map[string]any{"privacy": 1}
	}
	return map[string]any{
		"profile":               profile,
		"characters":            chars,
		"characterActivities":   map[string]any{"privacy": 1, "data": map[string]any{cid: activity}},
		"profileTransitoryData": transitory,
	}
}

func card(p *player) map[string]any {
	return map[string]any{
		"membershipType": p.mType, "membershipId": p.id, "crossSaveOverride": 0,
		"bungieGlobalDisplayName": p.name, "bungieGlobalDisplayNameCode": p.code,
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName     string `json:"displayName"`
		DisplayNameCode int    `json:"displayNameCode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBungieError(w, 7, "ParameterParseFailure")
		return
	}
	cards := []map[string]any{}
	s.mu.Lock()
	for _, p := range s.players {
		if strings.EqualFold(p.name, body.DisplayName) && p.code == body.DisplayNameCode {
			cards = append(cards, card(p))
		}
	}
	s.mu.Unlock()
	writeOK(w, cards)
}

func details(id string, l lobby) map[string]any {
	return map[string]any{"referenceId": l.mapHash, "directorActivityHash": CompetitiveHash, "instanceId": id, "mode": 69, "modes": []int{5, 69}}
}

// handleHistory lists the lobbies the player was in, newest first, up to count. Each fake
// player has one character, so the character ID is not checked.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count < 1 {
		count = 25
	}
	player := r.PathValue("id")
	acts := []map[string]any{}
	for _, id := range []string{LastMatchID, OlderMatchID} {
		l := s.lobbies[id]
		if len(acts) == count || !slices.ContainsFunc(l.members, func(m member) bool { return m.id == player }) {
			continue
		}
		acts = append(acts, map[string]any{
			"period":          l.period,
			"activityDetails": details(id, l),
			"values": map[string]any{"activityDurationSeconds": map[string]any{
				"statId": "activityDurationSeconds",
				"basic":  map[string]any{"value": l.duration.Seconds(), "displayValue": l.duration.String()},
			}},
		})
	}
	writeOK(w, map[string]any{"activities": acts})
}

func (s *Server) handlePGCR(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	l, ok := s.lobbies[id]
	if !ok {
		writeBungieError(w, bungie.CodeAccountNotFound, "ActivityNotFound")
		return
	}
	entries := []map[string]any{}
	s.mu.Lock()
	for _, m := range l.members {
		p := s.players[m.id]
		info := card(p)
		info["iconPath"] = p.emblem()
		entries = append(entries, map[string]any{
			"characterId": p.characterID(),
			"player":      map[string]any{"destinyUserInfo": info, "characterClass": "Hunter"},
			"values":      map[string]any{"team": map[string]any{"basic": map[string]any{"value": m.team, "displayValue": ""}}},
		})
	}
	s.mu.Unlock()
	writeOK(w, map[string]any{"period": l.period, "activityDetails": details(id, l), "entries": entries})
}

func (s *Server) handleSetState(w http.ResponseWriter, r *http.Request) {
	var body struct {
		State State `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validState(body.State) {
		http.Error(w, "state must be one of orbit, match, pve, offline, private", http.StatusBadRequest)
		return
	}
	if err := s.SetState(r.PathValue("id"), body.State); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, response any) {
	writeJSON(w, map[string]any{"Response": response, "ErrorCode": bungie.CodeSuccess, "ErrorStatus": "Success", "Message": "Ok"})
}

func writeBungieError(w http.ResponseWriter, code int, status string) {
	writeJSON(w, map[string]any{"ErrorCode": code, "ErrorStatus": status, "Message": status})
}
