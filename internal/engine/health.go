package engine

import "time"

// APIState is Bungie's health as seen by the poller; it drives the dashboard banner.
type APIState string

const (
	APIOK       APIState = "ok"
	APIDegraded APIState = "degraded"
	APIDown     APIState = "down"
)

// minHealthSamples is how many recent polls are needed before failures count as degraded.
const minHealthSamples = 4

// downAfterFailures consecutive failed polls mean Bungie is down even without SystemDisabled.
const downAfterFailures = 5

type sample struct {
	at     time.Time
	failed bool
}

// health tracks recent poll outcomes. Not safe for concurrent use; Engine guards it.
type health struct {
	window  time.Duration
	samples []sample
	down    bool
	// consecutive counts failed polls since the last success.
	consecutive int
}

// record notes one poll outcome. disabled means Bungie reported SystemDisabled (maintenance).
func (h *health) record(at time.Time, failed, disabled bool) {
	h.samples = append(h.samples, sample{at: at, failed: failed})
	if failed {
		h.consecutive++
	} else {
		h.consecutive = 0
	}
	switch {
	case disabled, failed && h.consecutive >= downAfterFailures:
		h.down = true
	case !failed:
		h.down = false
	}
}

func (h *health) state(now time.Time) APIState {
	cut := 0
	for cut < len(h.samples) && now.Sub(h.samples[cut].at) > h.window {
		cut++
	}
	h.samples = h.samples[cut:]
	if h.down {
		return APIDown
	}
	failures := 0
	for _, s := range h.samples {
		if s.failed {
			failures++
		}
	}
	if len(h.samples) >= minHealthSamples && failures*2 > len(h.samples) {
		return APIDegraded
	}
	return APIOK
}
