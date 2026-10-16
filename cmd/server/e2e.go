//go:build e2e

package main

import (
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/engine"
)

// e2e builds poll every 2 seconds so a scripted status change reaches the board quickly.
func init() {
	e2eBuild = true
	engineConfig = func() engine.Config {
		c := engine.DefaultConfig()
		c.Interval = 2 * time.Second
		c.RefreshCooldown = 2 * time.Second
		c.RefreshSkip = 0
		return c
	}
}
