// Command probe polls real Destiny 2 profiles and records labeled raw responses.
//
//	BUNGIE_API_KEY=... go run ./cmd/probe -players "You#1234,Friend#5678"
//
// Type a label and press Enter at any time to tag the following snapshots.
package main

import (
	"bufio"
	"context"
	"errors"
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
	players := flag.String("players", "", `comma-separated Bungie Names, e.g. "You#1234,Friend#5678"`)
	interval := flag.Duration("interval", 15*time.Second, "poll interval")
	out := flag.String("out", "testdata/probe", "output directory")
	flag.Parse()
	if *players == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *interval <= 0 {
		fmt.Fprintln(os.Stderr, "-interval must be greater than 0")
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

	type parsed struct {
		raw  string
		name string
		code int
	}
	var parsedNames []parsed
	for _, raw := range names {
		name, code, err := bungie.ParseBungieName(raw)
		if err != nil {
			return err
		}
		parsedNames = append(parsedNames, parsed{raw, name, code})
	}

	client := bungie.NewClient(cfg.BungieAPIKey, cfg.BungieBaseURL, nil)
	log.Println("loading manifest (downloads tens of MB)...")
	m, err := manifest.Load(ctx, client)
	if err != nil {
		return err
	}
	log.Printf("manifest %s loaded", m.Version)

	var targets []target
	for i, pn := range parsedNames {
		card, err := client.SearchByBungieName(ctx, pn.name, pn.code)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", pn.raw, err)
		}
		targets = append(targets, target{name: bungie.FormatBungieName(pn.name, pn.code), card: card})
		log.Printf("resolved %s → type=%d id=%s", pn.raw, card.MembershipType, card.MembershipID)
		log.Printf("P%d = %s", i+1, targets[i].name)
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
		pollAll(ctx, client, m, rec, &label, targets)
		select {
		case <-ctx.Done():
			stop()
			return nil
		case <-ticker.C:
		}
	}
}

func pollAll(ctx context.Context, client *bungie.Client, m *manifest.Manifest, rec *probe.Recorder, label *probe.Label, targets []target) {
	for _, t := range targets {
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		lbl := label.Get()
		p, err := client.GetProfile(ctx, t.card.MembershipType, t.card.MembershipID)
		took := time.Since(now).Round(time.Millisecond)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				continue
			}
			log.Printf("%s: %v throttled=%t took=%s", t.name, err, bungie.IsThrottled(err), took)
			continue
		}
		s := probe.Summarize(t.name, p, m)
		if err := rec.Record(now, lbl, s, p.Raw); err != nil {
			log.Printf("%s: record: %v", t.name, err)
		}
		fmt.Printf("%s [%s] %s took=%s\n", now.Format("15:04:05"), lbl, s, took)
	}
}

func readLabels(r io.Reader, l *probe.Label) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l.Set(sc.Text())
		log.Printf("label → %q (set at %s)", l.Get(), time.Now().Format("15:04:05.000"))
	}
}
