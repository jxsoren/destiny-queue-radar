package manifest

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Config tunes a Live manifest. Zero values take the defaults.
type Config struct {
	CheckInterval    time.Duration    // how often to compare versions; default 1h
	MinNudgeInterval time.Duration    // min gap since the last check before an unknown hash may trigger one; default 5m
	Now              func() time.Time // clock; default time.Now
}

// Live serves lookups from the current manifest and swaps in a new one when Bungie's
// manifest version changes (spec §7.2).
type Live struct {
	cur       atomic.Pointer[Manifest]
	src       Source
	log       *slog.Logger
	cfg       Config
	nudge     chan struct{}
	lastCheck atomic.Int64 // unix nanos of the last version check (or of construction)
}

// NewLive wraps an already loaded manifest.
func NewLive(m *Manifest, src Source, log *slog.Logger, cfg Config) *Live {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = time.Hour
	}
	if cfg.MinNudgeInterval <= 0 {
		cfg.MinNudgeInterval = 5 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	l := &Live{src: src, log: log, cfg: cfg, nudge: make(chan struct{}, 1)}
	l.cur.Store(m)
	l.lastCheck.Store(cfg.Now().UnixNano())
	return l
}

// Current returns the manifest in use right now.
func (l *Live) Current() *Manifest { return l.cur.Load() }

// Version returns the version of the manifest in use right now.
func (l *Live) Version() string { return l.cur.Load().Version }

func (l *Live) Activity(hash uint32) (Activity, bool) {
	a, ok := l.cur.Load().Activity(hash)
	if !ok {
		l.requestCheck(hash != 0)
	}
	return a, ok
}

func (l *Live) Mode(hash uint32) (Mode, bool) {
	m, ok := l.cur.Load().Mode(hash)
	if !ok {
		l.requestCheck(hash != 0)
	}
	return m, ok
}

// ModeByType finds a mode by DestinyActivityModeType. Type 0 is "none" and never nudges.
func (l *Live) ModeByType(modeType int) (Mode, bool) {
	m, ok := l.cur.Load().ModeByType(modeType)
	if !ok {
		l.requestCheck(modeType != 0)
	}
	return m, ok
}

// requestCheck asks Run for an early version check, at most once per MinNudgeInterval.
// It never blocks.
func (l *Live) requestCheck(nonZero bool) {
	if !nonZero {
		return
	}
	if l.cfg.Now().UnixNano()-l.lastCheck.Load() < int64(l.cfg.MinNudgeInterval) {
		return
	}
	select {
	case l.nudge <- struct{}{}:
	default:
	}
}

// Run checks for a new manifest every CheckInterval, and early when a lookup misses, until ctx is done.
func (l *Live) Run(ctx context.Context) {
	t := time.NewTicker(l.cfg.CheckInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-l.nudge:
		}
		l.check(ctx)
	}
}

func (l *Live) check(ctx context.Context) {
	l.lastCheck.Store(l.cfg.Now().UnixNano())
	old := l.Version()
	info, err := l.src.GetManifestInfo(ctx)
	if err != nil {
		if ctx.Err() == nil {
			l.log.Warn("manifest version check failed; keeping current manifest", "version", old, "err", err)
		}
		return
	}
	if info.Version == old {
		return
	}
	m, err := Load(ctx, l.src)
	if err != nil {
		if ctx.Err() == nil {
			l.log.Warn("manifest reload failed; keeping current manifest", "version", old, "err", err)
		}
		return
	}
	if m.Version == old {
		return
	}
	l.cur.Store(m)
	l.log.Info("manifest reloaded", "old_version", old, "new_version", m.Version)
}
