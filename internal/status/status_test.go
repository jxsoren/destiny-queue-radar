package status

import (
	"encoding/json"
	"reflect"
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

// TestEqual_CoversEveryField guards against adding a Status field without comparing it in Equal.
func TestEqual_CoversEveryField(t *testing.T) {
	t0 := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	base := Status{
		State: InMatch, Place: PlaceOrbit, Activity: "a", Mode: "m", Playlist: "p",
		StartedAt: ptr(t0), LastPvPEndedAt: ptr(t0), LastPvPPlaylist: "l",
	}
	typ := reflect.TypeOf(Status{})
	if typ.NumField() != 8 {
		t.Fatalf("Status has %d fields, want 8: update Equal and this test", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		changed := base
		fv := reflect.ValueOf(&changed).Elem().Field(i)
		switch fv.Interface().(type) {
		case string:
			fv.SetString("changed")
		case State:
			fv.Set(reflect.ValueOf(Offline))
		case Place:
			fv.Set(reflect.ValueOf(PlacePvE))
		case *time.Time:
			fv.Set(reflect.ValueOf(ptr(t0.Add(time.Hour))))
		default:
			t.Fatalf("field %s has unhandled type %s: extend this test", f.Name, f.Type)
		}
		if base.Equal(changed) {
			t.Errorf("Equal ignores field %s", f.Name)
		}
	}
}
