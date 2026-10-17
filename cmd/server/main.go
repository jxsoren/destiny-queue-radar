// Command server runs Destiny Queue Radar: the API, live stream, Bungie sign-in and poller.
//
//	set -a; . ./.env; set +a
//	go run ./cmd/server
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/auth"
	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/classifier"
	"github.com/jxsoren/destiny-queue-radar/internal/config"
	"github.com/jxsoren/destiny-queue-radar/internal/cryptobox"
	"github.com/jxsoren/destiny-queue-radar/internal/engine"
	"github.com/jxsoren/destiny-queue-radar/internal/hub"
	"github.com/jxsoren/destiny-queue-radar/internal/logging"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/ratelimit"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
	"github.com/jxsoren/destiny-queue-radar/internal/web"
	"github.com/jxsoren/destiny-queue-radar/internal/webui"
)

const (
	bungieRequestsPerSecond = 20
	unwatchedPlayerTTL      = 30 * 24 * time.Hour // spec §7.5
	cleanupInterval         = time.Hour
)

// engineConfig is the poller configuration; e2e builds replace it with a faster one.
var engineConfig = engine.DefaultConfig

// e2eBuild is set by e2e.go in binaries built with -tags e2e.
var e2eBuild = false

func main() {
	log, flush, err := logging.Setup(os.Getenv, os.Stdout)
	if err != nil {
		slog.Error("logging setup failed", "err", err)
		os.Exit(1)
	}
	// Packages that log through slog.Default (internal/auth) get the same output and Sentry.
	slog.SetDefault(log)
	err = run(log, flush)
	if err != nil {
		log.Error("server stopped", "err", err)
	}
	flush()
	if err != nil {
		os.Exit(1)
	}
}

func run(log *slog.Logger, flush func()) error {
	cfg, err := config.ServerFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	limiter := ratelimit.New(bungieRequestsPerSecond)
	client := bungie.NewClient(cfg.BungieAPIKey, cfg.BungieBaseURL, nil)
	client.SetLimiter(limiter)
	stats := bungie.NewClient(cfg.BungieAPIKey, cfg.StatsBaseURL, nil)
	stats.SetLimiter(limiter)

	log.Info("loading manifest")
	man, err := manifest.Load(ctx, client)
	if err != nil {
		return err
	}
	log.Info("manifest loaded", "version", man.Version)
	live := manifest.NewLive(man, client, log, manifest.Config{})

	box, err := cryptobox.New(cfg.TokenKey)
	if err != nil {
		return err
	}
	tokens := auth.NewTokenManager(st, box, cfg.OAuthClientID, cfg.OAuthClientSecret, cfg.BungieBaseURL, nil)
	h := hub.New(32)
	eng := engine.New(engineConfig(), st, client, client, classifier.New(live), h, log)
	// Closed when Shutdown begins, so open SSE streams end while normal requests drain.
	streamsDone := make(chan struct{})
	handler := web.New(web.Deps{
		Store:         st,
		Engine:        eng,
		Hub:           h,
		Bungie:        client,
		Tokens:        tokens,
		Matches:       matches.New(client, client, stats, live),
		Static:        webui.FS(),
		OAuthClientID: cfg.OAuthClientID,
		Done:          streamsDone,
		Log:           log,
	})

	if !webui.Built() {
		log.Warn("serving the placeholder page; run `make web` and rebuild to embed the dashboard")
	}
	if e2eBuild {
		log.Warn("E2E BUILD: test sign-in at /auth/e2e-login is enabled; never deploy this binary")
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	srv.RegisterOnShutdown(func() { close(streamsDone) })

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		defer reportPanic(log, flush)
		eng.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		defer reportPanic(log, flush)
		live.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		defer reportPanic(log, flush)
		cleanup(ctx, st, log)
	}()

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "tls", cfg.TLSCertFile != "")
		if cfg.TLSCertFile != "" {
			serveErr <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			log.Warn("serving plain HTTP; Bungie sign-in needs HTTPS, from a TLS proxy (as on Railway) or TLS_CERT_FILE and TLS_KEY_FILE")
			serveErr <- srv.ListenAndServe()
		}
	}()

	select {
	case err := <-serveErr:
		stop()
		wg.Wait()
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// reportPanic logs a background goroutine's panic and flushes it to Sentry, then re-panics:
// the process still crashes, and the platform restarts it.
func reportPanic(log *slog.Logger, flush func()) {
	v := recover()
	if v == nil {
		return
	}
	log.Error("background panic", "err", fmt.Errorf("panic: %v", v), "stack", string(debug.Stack()))
	flush()
	panic(v)
}

// cleanup deletes long-unwatched players and expired sessions every hour.
func cleanup(ctx context.Context, st *store.Store, log *slog.Logger) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		now := time.Now()
		if n, err := st.DeleteUnwatchedPlayers(ctx, now.Add(-unwatchedPlayerTTL)); err != nil && ctx.Err() == nil {
			log.Error("cleanup players", "err", err)
		} else if n > 0 {
			log.Info("deleted unwatched players", "count", n)
		}
		if n, err := st.DeleteExpiredSessions(ctx, now); err != nil && ctx.Err() == nil {
			log.Error("cleanup sessions", "err", err)
		} else if n > 0 {
			log.Info("deleted expired sessions", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
