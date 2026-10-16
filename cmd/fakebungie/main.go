// Command fakebungie serves a scriptable fake Bungie.net API for end-to-end tests and for running
// the dashboard without an API key. Point BUNGIE_BASE_URL and BUNGIE_STATS_BASE_URL at it.
//
//	go run ./cmd/fakebungie -addr 127.0.0.1:9100
//	curl -X POST -d '{"state":"match"}' http://127.0.0.1:9100/fake/players/4611686018400000002/state
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/fakebungie"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9100", "listen address")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	log.Info("fake Bungie API listening", "addr", *addr)
	srv := &http.Server{Addr: *addr, Handler: fakebungie.New(), ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Error("fake Bungie API stopped", "err", err)
		os.Exit(1)
	}
}
