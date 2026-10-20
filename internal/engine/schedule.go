package engine

import (
	"sort"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/status"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
)

// entry is the scheduler's view of one player in the active set.
type entry struct {
	player     store.Player
	lastPolled time.Time
	manualAt   time.Time // non-zero while a manual refresh is pending
	manualOp   string    // request ID of the pending manual refresh
	pollOp     string    // request ID of the poll in flight
	inFlight   bool
	lastMinted time.Time // newest responseMintedTimestamp seen; resets when the entry is dropped
}

// pickDue returns up to limit players to poll now: manual refreshes first, then players whose
// interval has elapsed, oldest poll first. When demand exceeds capacity, every player's
// interval stretches evenly because the oldest always goes next.
func pickDue(entries map[string]*entry, now time.Time, interval time.Duration, limit int) []string {
	type candidate struct {
		id     string
		manual bool
		last   time.Time
	}
	var cs []candidate
	for id, e := range entries {
		if e.inFlight {
			continue
		}
		manual := !e.manualAt.IsZero()
		if !manual && e.player.Status.State == status.NotFound {
			continue // terminal: only a manual refresh or Recheck re-checks it
		}
		if !manual && now.Sub(e.lastPolled) < interval {
			continue
		}
		cs = append(cs, candidate{id: id, manual: manual, last: e.lastPolled})
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].manual != cs[j].manual {
			return cs[i].manual
		}
		if !cs[i].last.Equal(cs[j].last) {
			return cs[i].last.Before(cs[j].last)
		}
		return cs[i].id < cs[j].id
	})
	if len(cs) > limit {
		cs = cs[:limit]
	}
	ids := make([]string, len(cs))
	for i, c := range cs {
		ids[i] = c.id
	}
	return ids
}
