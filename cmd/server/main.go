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
	"net"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/matches"
	"github.com/jxsoren/destiny-queue-radar/internal/ratelimit"
	"github.com/jxsoren/destiny-queue-radar/internal/store"
	"github.com/jxsoren/destiny-queue-radar/internal/web"
)

const (
	bungieRequestsPerSecond = 20
	unwatchedPlayerTTL      = 30 * 24 * time.Hour // spec §7.5
	cleanupInterval         = time.Hour
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
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

	box, err := cryptobox.New(cfg.TokenKey)
	if err != nil {
		return err
	}
	tokens := auth.NewTokenManager(st, box, cfg.OAuthClientID, cfg.OAuthClientSecret, cfg.BungieBaseURL, nil)
	h := hub.New(32)
	eng := engine.New(engine.DefaultConfig(), st, client, classifier.New(man), h, log)
	handler := web.New(web.Deps{
		Store:         st,
		Engine:        eng,
		Hub:           h,
		Bungie:        client,
		Tokens:        tokens,
		Matches:       matches.New(client, client, stats, man),
		OAuthClientID: cfg.OAuthClientID,
		Log:           log,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// Request contexts end on shutdown, so open SSE streams close promptly.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		eng.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		cleanup(ctx, st, log)
	}()

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "tls", cfg.TLSCertFile != "")
		if cfg.TLSCertFile != "" {
			serveErr <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			log.Warn("serving plain HTTP; Bungie sign-in needs HTTPS (set TLS_CERT_FILE and TLS_KEY_FILE)")
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
