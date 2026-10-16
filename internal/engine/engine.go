// Package engine keeps watched players' statuses fresh: it polls Bungie for players watched
// by online users, classifies each profile, stores the result and publishes changes.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// Store is the subset of *store.Store the engine uses.
type Store interface {
	WatchedBy(ctx context.Context, userIDs []int64) ([]store.Player, error)
	WatchersOf(ctx context.Context, membershipID string) ([]int64, error)
	Player(ctx context.Context, membershipID string) (store.Player, error)
	SavePollSuccess(ctx context.Context, membershipID string, st status.Status, emblemPath, emblemBackgroundPath string, at time.Time) error
	SavePollFailure(ctx context.Context, membershipID string, at time.Time) error
}

// Profiles is implemented by *bungie.Client.
type Profiles interface {
	GetProfile(ctx context.Context, membershipType int64, membershipID string) (*bungie.Profile, error)
}

// Hub is the subset of *hub.Hub the engine uses.
type Hub interface {
	ActiveUsers(grace time.Duration) []int64
	SendTo(userIDs []int64, ev hub.Event)
	Broadcast(ev hub.Event)
}

var (
	_ Store    = (*store.Store)(nil)
	_ Profiles = (*bungie.Client)(nil)
	_ Hub      = (*hub.Hub)(nil)
)

const (
	EventPlayerUpdate = "player_update"
	EventAPIStatus    = "api_status"
)

// PlayerUpdate is the payload of a player_update event.
type PlayerUpdate struct {
	MembershipID string        `json:"membershipId"`
	Status       status.Status `json:"status"`
	PolledAt     time.Time     `json:"polledAt"`
}

// APIStatusUpdate is the payload of an api_status event.
type APIStatusUpdate struct {
	Bungie APIState `json:"bungie"`
}

// Config tunes the engine. New clamps unusable values: Workers and Tick to at least 1 (Tick
// defaults to one second when <= 0), and Interval, DownProbe and RefreshCooldown to non-negative.
type Config struct {
	Interval        time.Duration // routine poll interval per player
	Grace           time.Duration // how long a disconnected user's players stay active
	Workers         int           // concurrent polls
	RefreshCooldown time.Duration // minimum time between one user's manual refreshes
	RefreshSkip     time.Duration // manual refresh skips players polled more recently than this
	DownProbe       time.Duration // while Bungie is down, poll at most one player this often
	Tick            time.Duration // scheduler resolution
}

func DefaultConfig() Config {
	return Config{
		Interval:        60 * time.Second,
		Grace:           5 * time.Minute,
		Workers:         8,
		RefreshCooldown: 30 * time.Second,
		RefreshSkip:     10 * time.Second,
		DownProbe:       60 * time.Second,
		Tick:            time.Second,
	}
}

// CooldownError is returned by RequestRefresh when the user refreshed too recently.
type CooldownError struct {
	RetryAfter time.Duration
}

func (e *CooldownError) Error() string {
	return fmt.Sprintf("engine: refresh on cooldown, retry after %s", e.RetryAfter.Round(time.Second))
}

type Engine struct {
	cfg        Config
	store      Store
	profiles   Profiles
	classifier status.Classifier
	hub        Hub
	log        *slog.Logger
	now        func() time.Time

	mu          sync.Mutex
	entries     map[string]*entry
	lastRefresh map[int64]time.Time
	health      health
	apiState    APIState
	lastProbe   time.Time
}

func New(cfg Config, st Store, p Profiles, c status.Classifier, h Hub, log *slog.Logger) *Engine {
	cfg.Workers = max(cfg.Workers, 1)
	if cfg.Tick <= 0 {
		cfg.Tick = time.Second
	}
	cfg.Interval = max(cfg.Interval, 0)
	cfg.DownProbe = max(cfg.DownProbe, 0)
	cfg.RefreshCooldown = max(cfg.RefreshCooldown, 0)
	return &Engine{
		cfg:         cfg,
		store:       st,
		profiles:    p,
		classifier:  c,
		hub:         h,
		log:         log,
		now:         time.Now,
		entries:     map[string]*entry{},
		lastRefresh: map[int64]time.Time{},
		health:      health{window: time.Minute},
		apiState:    APIOK,
	}
}

// APIState reports Bungie's current health, e.g. for a new dashboard's initial snapshot.
func (e *Engine) APIState() APIState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.apiState
}

// Run schedules and performs polls until ctx is cancelled, then waits for in-flight polls.
func (e *Engine) Run(ctx context.Context) error {
	jobs := make(chan string, e.cfg.Workers)
	var wg sync.WaitGroup
	for range e.cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				e.poll(ctx, id)
			}
		}()
	}
	defer func() {
		close(jobs)
		wg.Wait()
	}()

	ticker := time.NewTicker(e.cfg.Tick)
	defer ticker.Stop()
	for {
		if err := e.refreshActiveSet(ctx); err != nil && ctx.Err() == nil {
			e.log.Error("refresh active set", "err", err)
		}
		e.dispatch(jobs)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// RequestRefresh queues an immediate poll of every player the user watches, except players
// polled within RefreshSkip. It returns *CooldownError if called again within RefreshCooldown.
func (e *Engine) RequestRefresh(ctx context.Context, userID int64) error {
	e.mu.Lock()
	now := e.now()
	prevRefresh, hadRefresh := e.lastRefresh[userID]
	if hadRefresh {
		if wait := e.cfg.RefreshCooldown - now.Sub(prevRefresh); wait > 0 {
			e.mu.Unlock()
			return &CooldownError{RetryAfter: wait}
		}
	}
	for id, t := range e.lastRefresh {
		if now.Sub(t) >= e.cfg.RefreshCooldown {
			delete(e.lastRefresh, id)
		}
	}
	e.lastRefresh[userID] = now
	e.mu.Unlock()

	players, err := e.store.WatchedBy(ctx, []int64{userID})
	if err != nil {
		// Don't let a failed refresh consume the user's cooldown.
		e.mu.Lock()
		if hadRefresh {
			e.lastRefresh[userID] = prevRefresh
		} else {
			delete(e.lastRefresh, userID)
		}
		e.mu.Unlock()
		return fmt.Errorf("engine: load watchlist: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range players {
		en := e.entryLocked(p)
		if en.inFlight || now.Sub(en.lastPolled) < e.cfg.RefreshSkip {
			continue
		}
		en.manualAt = now
	}
	return nil
}

// entryLocked returns the scheduler entry for p, creating it from the stored poll time.
func (e *Engine) entryLocked(p store.Player) *entry {
	en, ok := e.entries[p.MembershipID]
	if !ok {
		en = &entry{}
		if p.LastPolledAt != nil {
			en.lastPolled = *p.LastPolledAt
		}
		e.entries[p.MembershipID] = en
	}
	// The engine is the only writer of Status, so keep what it last learned; the snapshot p
	// may predate a poll that completed meanwhile.
	prevStatus := en.player.Status
	en.player = p
	if ok {
		en.player.Status = prevStatus
	}
	return en
}

func (e *Engine) refreshActiveSet(ctx context.Context) error {
	players, err := e.store.WatchedBy(ctx, e.hub.ActiveUsers(e.cfg.Grace))
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	keep := make(map[string]bool, len(players))
	for _, p := range players {
		keep[p.MembershipID] = true
		e.entryLocked(p)
	}
	for id, en := range e.entries {
		// A pending manual refresh keeps its entry even if no watcher is online.
		if !keep[id] && !en.inFlight && en.manualAt.IsZero() {
			delete(e.entries, id)
		}
	}
	return nil
}

func (e *Engine) dispatch(jobs chan<- string) {
	e.mu.Lock()
	now := e.now()
	limit := cap(jobs) - len(jobs)
	probing := false
	switch e.apiState {
	case APIDown:
		if now.Sub(e.lastProbe) < e.cfg.DownProbe {
			limit = 0
		} else {
			limit = min(limit, 1)
			probing = true
		}
	case APIDegraded:
		// Spec 7.2: slow polling down while Bungie is struggling.
		limit = min(limit, max(1, cap(jobs)/4))
	}
	ids := pickDue(e.entries, now, e.cfg.Interval, limit)
	if probing && len(ids) > 0 {
		e.lastProbe = now // an empty probe must not consume the slot
	}
	for _, id := range ids {
		e.entries[id].inFlight = true
		e.entries[id].manualAt = time.Time{}
	}
	e.mu.Unlock()
	for _, id := range ids {
		jobs <- id
	}
}

func (e *Engine) poll(ctx context.Context, id string) {
	defer e.finish(id)

	e.mu.Lock()
	en, ok := e.entries[id]
	var membershipType int64
	if ok {
		membershipType = en.player.MembershipType
	}
	e.mu.Unlock()
	if !ok {
		return
	}

	prev, err := e.store.Player(ctx, id)
	if err != nil {
		if ctx.Err() == nil {
			e.log.Error("load player", "player", id, "err", err)
		}
		return
	}
	now := e.now()
	profile, err := e.profiles.GetProfile(ctx, membershipType, id)
	if ctx.Err() != nil {
		return
	}
	e.recordHealth(now, err)

	var next status.Status
	switch {
	case err == nil:
		var decided bool
		next, decided = e.classifier.Classify(profile, prev.Status, now)
		if !decided {
			// The API call worked but the profile could not be classified; keep the previous
			// status and do not advance last_success_at, so the dashboard shows it as stale.
			e.log.Debug("profile undecidable", "player", id)
			e.savePollFailure(ctx, id, now)
			return
		}
	case bungie.IsNotFound(err):
		next = status.Status{State: status.NotFound}
	default:
		e.log.Warn("poll failed", "player", id, "err", err, "throttled", bungie.IsThrottled(err))
		e.savePollFailure(ctx, id, now)
		return
	}

	emblem, background := activeEmblem(profile)
	if err := e.store.SavePollSuccess(ctx, id, next, emblem, background, now); err != nil {
		// ErrNotFound: the player was deleted mid-poll; nothing to save or publish.
		if ctx.Err() == nil && !errors.Is(err, store.ErrNotFound) {
			e.log.Error("save poll", "player", id, "err", err)
		}
		return
	}
	e.mu.Lock()
	if en, ok := e.entries[id]; ok {
		en.player.Status = next // lets the scheduler see terminal states such as NOT_FOUND
	}
	e.mu.Unlock()
	if !next.Equal(prev.Status) {
		e.publish(ctx, id, next, now)
	}
}

func (e *Engine) savePollFailure(ctx context.Context, id string, now time.Time) {
	if err := e.store.SavePollFailure(ctx, id, now); err != nil && ctx.Err() == nil && !errors.Is(err, store.ErrNotFound) {
		e.log.Error("save poll failure", "player", id, "err", err)
	}
}

func (e *Engine) finish(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if en, ok := e.entries[id]; ok {
		en.inFlight = false
		// lastPolled advances even if the poll ended early on a store error, so that player
		// also waits a full Interval.
		// lastPolled is when the poll completed, not started. This is deliberate: under load the
		// interval stretches slightly. A failed poll also waits a full Interval before retrying.
		en.lastPolled = e.now()
	}
}

func (e *Engine) publish(ctx context.Context, id string, st status.Status, at time.Time) {
	watchers, err := e.store.WatchersOf(ctx, id)
	if err != nil {
		if ctx.Err() == nil {
			e.log.Error("load watchers", "player", id, "err", err)
		}
		return
	}
	e.hub.SendTo(watchers, hub.Event{
		Type: EventPlayerUpdate,
		Data: PlayerUpdate{MembershipID: id, Status: st, PolledAt: at},
	})
}

func (e *Engine) recordHealth(at time.Time, err error) {
	// Throttling means we are rate-limited, not that Bungie is unhealthy; not-found is a
	// successful answer about a missing player.
	throttled := bungie.IsThrottled(err) && !bungie.IsSystemDisabled(err)
	failed := err != nil && !bungie.IsNotFound(err)
	// Hold the lock across the Broadcast so concurrent workers cannot deliver state changes out
	// of order and leave clients on a stale state. hub.Broadcast never blocks or calls back.
	e.mu.Lock()
	defer e.mu.Unlock()
	if !throttled {
		e.health.record(at, failed, bungie.IsSystemDisabled(err))
	}
	// Even when throttled (no sample recorded) re-evaluate, so a stale degraded state decays
	// once its samples age out of the window.
	state := e.health.state(at)
	if state == e.apiState {
		return
	}
	e.apiState = state
	if state == APIDown {
		e.lastProbe = at // the first probe waits a full DownProbe
	}
	e.log.Info("bungie api state changed", "state", state)
	e.hub.Broadcast(hub.Event{Type: EventAPIStatus, Data: APIStatusUpdate{Bungie: state}})
}

// activeEmblem returns the emblem of the character currently in use, falling back to the
// most recently played character when there is no activity data.
func activeEmblem(p *bungie.Profile) (string, string) {
	if p == nil {
		return "", ""
	}
	id, ok := p.ActiveCharacterID()
	if !ok {
		var latest time.Time
		for cid, c := range p.Characters.Data {
			if id == "" || c.DateLastPlayed.After(latest) || (c.DateLastPlayed.Equal(latest) && cid < id) {
				id, latest = cid, c.DateLastPlayed
			}
		}
	}
	c := p.Characters.Data[id]
	return c.EmblemPath, c.EmblemBackgroundPath
}
