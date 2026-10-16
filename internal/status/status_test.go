package status

import (
	"encoding/json"
	"testing"
	"time"
)

func ptr(t time.Time) *time.Time { return &t }

func TestEqual(t *testing.T) {
	t0 := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	base := Status{State: InMatch, Activity: "Burnout", Mode: "Control", Playlist: "Control: Matchmade", StartedAt: ptr(t0)}

	same := base
	same.StartedAt = ptr(t0.In(time.FixedZone("x", 3600))) // same instant, different zone
	if !base.Equal(same) {
		t.Error("same instant in another zone should be equal")
	}

	tests := map[string]Status{
		"state":    {State: NotInPvP, Activity: base.Activity, Mode: base.Mode, Playlist: base.Playlist, StartedAt: base.StartedAt},
		"activity": {State: InMatch, Activity: "Javelin-4", Mode: base.Mode, Playlist: base.Playlist, StartedAt: base.StartedAt},
		"started":  {State: InMatch, Activity: base.Activity, Mode: base.Mode, Playlist: base.Playlist, StartedAt: ptr(t0.Add(time.Minute))},
		"nil time": {State: InMatch, Activity: base.Activity, Mode: base.Mode, Playlist: base.Playlist},
	}
	for name, other := range tests {
		if base.Equal(other) {
			t.Errorf("%s: expected not equal", name)
		}
	}
	if !(Status{}).Equal(Status{}) {
		t.Error("zero statuses should be equal")
	}
}

func TestStatus_JSONOmitsEmpty(t *testing.T) {
	b, err := json.Marshal(Status{State: Offline})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"state":"OFFLINE"}` {
		t.Errorf("json = %s", b)
	}
	var back Status
	if err := json.Unmarshal(b, &back); err != nil || !back.Equal(Status{State: Offline}) {
		t.Errorf("round trip = %+v, %v", back, err)
	}
}
