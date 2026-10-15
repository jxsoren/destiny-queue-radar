// Command probe polls real Destiny 2 profiles and records labeled raw responses.
//
//	BUNGIE_API_KEY=... go run ./cmd/probe -players "Kwivor#1993,Clae™#0880"
//
// Type a label and press Enter at any time to tag the following snapshots.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/jxsoren/destiny-queue-radar/internal/bungie"
	"github.com/jxsoren/destiny-queue-radar/internal/config"
	"github.com/jxsoren/destiny-queue-radar/internal/manifest"
	"github.com/jxsoren/destiny-queue-radar/internal/probe"
)

type target struct {
	name string
	card bungie.UserInfoCard
}

func main() {
	players := flag.String("players", "", `comma-separated Bungie Names, e.g. "Kwivor#1993,Clae™#0880"`)
	interval := flag.Duration("interval", 15*time.Second, "poll interval")
	out := flag.String("out", "testdata/probe", "output directory")
	flag.Parse()
	if *players == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(strings.Split(*players, ","), *interval, *out); err != nil {
		log.Fatal(err)
	}
}

func run(names []string, interval time.Duration, out string) error {
	cfg, err := config.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := bungie.NewClient(cfg.BungieAPIKey, cfg.BungieBaseURL, nil)
	log.Println("loading manifest (downloads tens of MB)...")
	m, err := manifest.Load(ctx, client)
	if err != nil {
		return err
	}
	log.Printf("manifest %s loaded", m.Version)

	var targets []target
	for _, raw := range names {
		name, code, err := bungie.ParseBungieName(raw)
		if err != nil {
			return err
		}
		card, err := client.SearchByBungieName(ctx, name, code)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", raw, err)
		}
		targets = append(targets, target{name: bungie.FormatBungieName(name, code), card: card})
		log.Printf("resolved %s → type=%d id=%s", raw, card.MembershipType, card.MembershipID)
	}

	rec, err := probe.NewRecorder(out)
	if err != nil {
		return err
	}
	defer rec.Close()

	var label probe.Label
	go readLabels(os.Stdin, &label)
	log.Printf("polling %d players every %s → %s", len(targets), interval, out)
	log.Println(`type a label + Enter to tag snapshots (e.g. "P1 orbit after comp"); Ctrl+C to stop`)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		pollAll(ctx, client, m, rec, label.Get(), targets)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func pollAll(ctx context.Context, client *bungie.Client, m *manifest.Manifest, rec *probe.Recorder, label string, targets []target) {
	now := time.Now()
	for _, t := range targets {
		p, err := client.GetProfile(ctx, t.card.MembershipType, t.card.MembershipID)
		if err != nil {
			log.Printf("%s: %v", t.name, err)
			continue
		}
		s := probe.Summarize(t.name, p, m)
		if err := rec.Record(now, label, s, p.Raw); err != nil {
			log.Printf("%s: record: %v", t.name, err)
		}
		fmt.Printf("%s [%s] %s\n", now.Format("15:04:05"), label, s)
	}
}

func readLabels(r io.Reader, l *probe.Label) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l.Set(sc.Text())
		log.Printf("label → %q", l.Get())
	}
}
