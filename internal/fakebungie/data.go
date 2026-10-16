package fakebungie

import "time"

// Manifest hashes served by the fake. Orbit's hash is the real one the classifier recognises.
const (
	OrbitHash           uint32 = 82913930
	BurnoutHash         uint32 = 1001
	JavelinHash         uint32 = 1002
	StrikeHash          uint32 = 1003
	CompetitiveHash     uint32 = 2001
	CompetitiveModeHash uint32 = 3001
	StrikeModeHash      uint32 = 3002
)

// The signed-in test user; internal/web's e2e sign-in stub creates this account.
const (
	SelfType = 3
	SelfID   = "4611686018400000001"
	SelfName = "Tester#0001"
)

// The test user's two recent PvP matches, newest first. Every lobby member has them in their
// history.
const (
	LastMatchID  = "9001"
	OlderMatchID = "9002"
)

// State is what a fake player is doing.
type State string

const (
	Orbit   State = "orbit"
	InMatch State = "match"
	PvE     State = "pve"
	Offline State = "offline"
	Private State = "private"
)

func validState(s State) bool {
	switch s {
	case Orbit, InMatch, PvE, Offline, Private:
		return true
	}
	return false
}

type player struct {
	mType int64
	id    string
	name  string
	code  int
	state State
	since time.Time
}

func (p *player) emblem() string {
	return "/common/destiny2_content/icons/fake_emblem_" + p.id[len(p.id)-2:] + ".jpg"
}

func (p *player) characterID() string { return "2305843009" + p.id[len(p.id)-9:] }

type member struct {
	id   string
	team int
}

type lobby struct {
	period   time.Time // start
	duration time.Duration
	mapHash  uint32
	members  []member
}

func roster(now time.Time) map[string]*player {
	list := []*player{
		{3, SelfID, "Tester", 1, Orbit, now},
		{3, "4611686018400000002", "Vex", 1111, Orbit, now},
		{3, "4611686018400000003", "Kestrel", 412, Offline, now},
		{3, "4611686018400000004", "Rook", 2093, PvE, now},
		{3, "4611686018400000005", "Halfmoon", 7781, InMatch, now.Add(-4 * time.Minute)},
		{3, "4611686018400000006", "Tallow", 5510, Private, now},
		{3, "4611686018400000007", "Mire", 3307, Orbit, now},
	}
	out := make(map[string]*player, len(list))
	for _, p := range list {
		out[p.id] = p
	}
	return out
}

func lobbies(now time.Time) map[string]lobby {
	return map[string]lobby{
		// Ended 7 minutes before startup: a cold start records it without making anyone
		// LIKELY_QUEUING (the window is 5 minutes).
		LastMatchID: {period: now.Add(-12 * time.Minute), duration: 5 * time.Minute, mapHash: BurnoutHash, members: []member{
			{SelfID, 16}, {"4611686018400000002", 16}, {"4611686018400000003", 16},
			{"4611686018400000004", 17}, {"4611686018400000005", 17}, {"4611686018400000006", 17},
		}},
		OlderMatchID: {period: now.Add(-50 * time.Minute), duration: 9 * time.Minute, mapHash: JavelinHash, members: []member{
			{SelfID, 16}, {"4611686018400000007", 16}, {"4611686018400000002", 17}, {"4611686018400000004", 17},
		}},
	}
}

type defProps struct {
	Name string `json:"name"`
}

type activityDef struct {
	Hash              uint32   `json:"hash"`
	DisplayProperties defProps `json:"displayProperties"`
	IsPvP             bool     `json:"isPvP"`
}

type modeDef struct {
	Hash                 uint32   `json:"hash"`
	DisplayProperties    defProps `json:"displayProperties"`
	ModeType             int      `json:"modeType"`
	ActivityModeCategory int      `json:"activityModeCategory"`
}

var activityDefs = map[string]activityDef{
	"82913930": {OrbitHash, defProps{"Orbit"}, false},
	"1001":     {BurnoutHash, defProps{"Burnout"}, true},
	"1002":     {JavelinHash, defProps{"Javelin-4"}, true},
	"1003":     {StrikeHash, defProps{"Vanguard Ops"}, false},
	"2001":     {CompetitiveHash, defProps{"Competitive"}, true},
}

var modeDefs = map[string]modeDef{
	"3001": {CompetitiveModeHash, defProps{"Competitive"}, 69, 2},
	"3002": {StrikeModeHash, defProps{"Vanguard Ops"}, 18, 1},
}
