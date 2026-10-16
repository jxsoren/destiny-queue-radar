package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
)

// scriptedSource is a mutable fake Source that reports every GetManifestInfo call on infoCalls.
type scriptedSource struct {
	mu        sync.Mutex
	info      bungie.ManifestInfo
	files     map[string]any
	infoErr   error
	fileErr   error
	fileCalls int
	infoCalls chan struct{}
}

func newScripted(version, actName string) *scriptedSource {
	s := &scriptedSource{infoCalls: make(chan struct{}, 100)}
	s.set(version, actName)
	return s
}

func (s *scriptedSource) set(version, actName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.info = bungie.ManifestInfo{Version: version, ActivityDefPath: "/a.json", ActivityModeDefPath: "/m.json"}
	s.files = map[string]any{
		"/a.json": map[string]any{
			"100": map[string]any{"hash": 100, "displayProperties": map[string]any{"name": "Burnout"}},
			"500": map[string]any{"hash": 500, "displayProperties": map[string]any{"name": actName}},
		},
		"/m.json": map[string]any{
			"300": map[string]any{"hash": 300, "displayProperties": map[string]any{"name": "Trials"}, "modeType": 84, "activityModeCategory": 2},
		},
	}
}

func (s *scriptedSource) setErrs(info, file error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infoErr, s.fileErr = info, file
}

func (s *scriptedSource) fileCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fileCalls
}

func (s *scriptedSource) GetManifestInfo(context.Context) (bungie.ManifestInfo, error) {
	s.mu.Lock()
	info, err := s.info, s.infoErr
	s.mu.Unlock()
	select {
	case s.infoCalls <- struct{}{}:
	default:
	}
	return info, err
}

func (s *scriptedSource) GetFile(_ context.Context, path string, out any) error {
	s.mu.Lock()
	s.fileCalls++
	err, f := s.fileErr, s.files[path]
	s.mu.Unlock()
	if err != nil {
		return err
	}
	b, _ := json.Marshal(f)
	return json.Unmarshal(b, out)
}

func (s *scriptedSource) waitInfo(t *testing.T) {
	t.Helper()
	select {
	case <-s.infoCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a manifest info check")
	}
}

func (s *scriptedSource) expectNoInfo(t *testing.T) {
	t.Helper()
	select {
	case <-s.infoCalls:
		t.Fatal("unexpected manifest info check")
	case <-time.After(30 * time.Millisecond):
	}
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// startLive loads v1 from src, wraps it, and runs it until the test ends.
func startLive(t *testing.T, src *scriptedSource, clock *fakeClock, tick time.Duration) (*Live, context.CancelFunc, chan struct{}) {
	t.Helper()
	m, err := Load(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	<-src.infoCalls // drain the initial load's call
	live := NewLive(m, src, nil, Config{CheckInterval: tick, MinNudgeInterval: 5 * time.Minute, Now: clock.Now})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { live.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return live, cancel, done
}

func TestLive_DelegatesLookups(t *testing.T) {
	src := newScripted("v1", "Orbit")
	m, _ := Load(context.Background(), src)
	live := NewLive(m, src, nil, Config{})
	if live.Version() != "v1" || live.Current() != m {
		t.Errorf("version=%q current=%p", live.Version(), live.Current())
	}
	if a, ok := live.Activity(100); !ok || a.Name != "Burnout" {
		t.Errorf("activity = %+v %v", a, ok)
	}
	if md, ok := live.Mode(300); !ok || md.Name != "Trials" {
		t.Errorf("mode = %+v %v", md, ok)
	}
	if md, ok := live.ModeByType(84); !ok || md.Name != "Trials" {
		t.Errorf("modeByType = %+v %v", md, ok)
	}
}

func TestLive_RunSwapsOnVersionChange(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, 5*time.Millisecond)
	src.set("v2", "NewRaid")
	deadline := time.Now().Add(2 * time.Second)
	for live.Version() != "v2" {
		if time.Now().After(deadline) {
			t.Fatal("manifest never swapped")
		}
		time.Sleep(time.Millisecond)
	}
	if a, ok := live.Activity(500); !ok || a.Name != "NewRaid" {
		t.Errorf("activity = %+v %v", a, ok)
	}
}

func TestLive_RunKeepsOldOnErrors(t *testing.T) {
	for name, errs := range map[string][2]error{
		"info": {errors.New("boom"), nil},
		"file": {nil, errors.New("boom")},
	} {
		t.Run(name, func(t *testing.T) {
			src := newScripted("v1", "Orbit")
			clock := &fakeClock{t: time.Unix(1000, 0)}
			live, _, _ := startLive(t, src, clock, time.Hour)
			src.set("v2", "NewRaid")
			src.setErrs(errs[0], errs[1])
			clock.Advance(10 * time.Minute)
			live.Activity(999) // nudge
			src.waitInfo(t)
			time.Sleep(20 * time.Millisecond)
			if live.Version() != "v1" {
				t.Errorf("version = %q, want v1 kept", live.Version())
			}
			if a, ok := live.Activity(500); !ok || a.Name != "Orbit" {
				t.Errorf("old definitions lost: %+v %v", a, ok)
			}
		})
	}
}

func TestLive_NoReloadWhenVersionUnchanged(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, time.Hour)
	before := src.fileCount()
	clock.Advance(10 * time.Minute)
	live.Activity(999)
	src.waitInfo(t)
	time.Sleep(20 * time.Millisecond)
	if got := src.fileCount(); got != before {
		t.Errorf("GetFile called %d times after unchanged check", got-before)
	}
}

func TestLive_UnknownActivityNudgesImmediately(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, time.Hour)
	clock.Advance(10 * time.Minute)
	if _, ok := live.Activity(777); ok {
		t.Fatal("hash 777 should be unknown")
	}
	src.waitInfo(t)
}

func TestLive_ZeroHashAndKnownHashDoNotNudge(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, time.Hour)
	clock.Advance(10 * time.Minute)
	live.Activity(0)
	live.Mode(0)
	live.Activity(100)
	src.expectNoInfo(t)
}

func TestLive_ModeMissNudges(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, time.Hour)
	clock.Advance(10 * time.Minute)
	live.ModeByType(12345)
	src.waitInfo(t)
}

func TestLive_RepeatedMissesWithinMinIntervalCheckOnce(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	live, _, _ := startLive(t, src, clock, time.Hour)
	clock.Advance(10 * time.Minute)
	live.Activity(777)
	src.waitInfo(t)
	time.Sleep(20 * time.Millisecond) // let Run record the check
	clock.Advance(time.Minute)
	for i := 0; i < 50; i++ {
		live.Activity(uint32(800 + i))
	}
	src.expectNoInfo(t)
	clock.Advance(5 * time.Minute)
	live.Activity(777)
	src.waitInfo(t)
}

func TestLive_RunExitsOnCancel(t *testing.T) {
	src := newScripted("v1", "Orbit")
	clock := &fakeClock{t: time.Unix(1000, 0)}
	_, cancel, done := startLive(t, src, clock, time.Hour)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}
