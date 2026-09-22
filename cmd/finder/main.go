package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/app"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/enrich"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sheets"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sources"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var version = "0.1.0"

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "finder:", e)
		os.Exit(1)
	}
}
func writeJSON(path string, value any) error {
	if path == "" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".finder-export-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if e = enc.Encode(value); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println("ThaiVtuberFinder " + version + "\nCommands: once, worker, import, export, compare, status, doctor, demo, healthcheck, enrich, proposals\nUse: finder <command> -config config/finder.json [-file path] [-out path]\nexport creates reviewed proposals; compare needs -canonical <bootstrap.json>.")
		return nil
	}
	if args[0] == "version" {
		fmt.Println(version)
		return nil
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	config := fs.String("config", "config/finder.json", "configuration JSON")
	file := fs.String("file", "", "import file path")
	kind := fs.String("kind", "jsonl", "import kind: jsonl, json, csv")
	out := fs.String("out", "", "output JSON file (default stdout)")
	canonical := fs.String("canonical", "", "canonical registry/bootstrap JSON")
	health := fs.String("url", "http://127.0.0.1:8080/healthz", "health check URL")
	limit := fs.Int("limit", 100, "enrich batch limit")
	offset := fs.Int("offset", 0, "enrich batch offset")
	platform := fs.String("platform", "bluesky", "enrich source platform")
	syncSheet := fs.Bool("sync-sheet", false, "sync to Google Sheet after enrich")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if args[0] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		r, e := client.Get(*health)
		if e != nil {
			return errors.New("health endpoint unavailable")
		}
		defer r.Body.Close()
		io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
		if r.StatusCode != 200 {
			return errors.New("health check failed")
		}
		return nil
	}
	if e := app.LoadEnv(".env"); e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if args[0] == "demo" {
		dir, e := os.MkdirTemp("", "finder-demo-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(dir)
		path := filepath.Join(dir, "input.jsonl")
		sample := "{\"url\":\"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa\",\"name\":\"Synthetic VTuberTH\"}\n{\"url\":\"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa\",\"name\":\"Synthetic renamed\"}\n{\"url\":\"https://twitch.tv/finder_synthetic_example\",\"name\":\"Synthetic PNGTuber\"}\n"
		if e = os.WriteFile(path, []byte(sample), 0600); e != nil {
			return e
		}
		c := app.Defaults()
		c.Database = filepath.Join(dir, "demo.sqlite")
		c.Sources = []sources.Config{{Name: "synthetic-demo", Kind: "jsonl", Path: path, Enabled: true}}
		a, e := app.New(c)
		if e != nil {
			return e
		}
		defer a.Close()
		r, e := a.Once(ctx)
		if e != nil {
			return e
		}
		return writeJSON(*out, r)
	}
	c, e := app.Load(*config)
	if e != nil {
		return e
	}
	if args[0] == "export" {
		net := netx.New()
		tokens, e := sheets.Credentials(net)
		if e != nil {
			return e
		}
		client := sheets.Client{HTTP: net, Tokens: tokens, SpreadsheetID: c.SpreadsheetID}
		doc, e := client.Export(ctx)
		if e != nil {
			return e
		}
		return writeJSON(*out, doc)
	}
	if args[0] == "import" {
		if *file == "" {
			return errors.New("import requires -file")
		}
		c.Sources = []sources.Config{{Name: "manual-import", Kind: *kind, Path: *file, Enabled: true, MaxItems: 20000}}
	}
	var a *app.App
	if args[0] == "status" || args[0] == "compare" || args[0] == "doctor" || args[0] == "proposals" || args[0] == "enrich" || args[0] == "verify" {
		a, e = app.NewUnlocked(c)
	} else {
		a, e = app.New(c)
	}
	if e != nil {
		return e
	}
	defer a.Close()
	switch args[0] {
	case "once", "import":
		r, e := a.Once(ctx)
		if outputErr := writeJSON(*out, r); outputErr != nil {
			return outputErr
		}
		return e
	case "worker":
		return a.Run(ctx, func(r app.Report) { _ = writeJSON("", r) })
	case "status":
		r, e := a.Store.Status()
		if e != nil {
			return e
		}
		return writeJSON(*out, r)
	case "compare":
		path := *canonical
		if path == "" {
			path = c.CanonicalFile
		}
		if path == "" {
			return errors.New("compare requires -canonical bootstrap.json")
		}
		known, _, e := sheets.LoadCanonical(path)
		if e != nil {
			return e
		}
		candidates, e := a.Store.Candidates()
		if e != nil {
			return e
		}
		return writeJSON(*out, app.CandidatesWithMatches(candidates, known))
	case "doctor":
		if e = a.Store.Ping(); e != nil {
			return e
		}
		if c.SheetSync {
			if _, e = a.Sheets.Tables(ctx); e != nil {
				return e
			}
		}
		return writeJSON(*out, map[string]any{"status": "ok", "version": version, "sheet_sync": c.SheetSync, "youtube_key_configured": a.Engine.APIKey != "", "note": "doctor validates connectivity/configuration; it does not prove discovery completeness"})
	case "enrich":
		targetPlatform := *platform
		if targetPlatform == "" {
			targetPlatform = "bluesky"
		}
		cands, err := a.Store.CandidatesByPlatform(targetPlatform)
		if err != nil {
			return fmt.Errorf("candidates by platform error: %w", err)
		}
		var knownCanonical map[string]string
		var knownInbox = map[string]string{}
		if a.Sheets != nil {
			tables, terr := a.Sheets.Tables(ctx)
			if terr == nil {
				kc, _, cerr := sheets.CanonicalKeys(tables["ACCOUNTS"], tables["ACCOUNT_LINKS"], tables["PERSONAS"])
				if cerr == nil {
					knownCanonical = kc
				}
				inbox := tables["FINDER_INBOX"]
				if len(inbox) > 1 {
					for _, r := range inbox[1:] {
						acc := sheets.RowAccount(r)
						if acc.URL != "" {
							knownInbox[acc.Key()] = r[0]
							knownInbox[acc.Platform+":url:"+acc.URL] = r[0]
						}
					}
				}
			}
		}
		allCands, _ := a.Store.Candidates()
		for _, c := range allCands {
			knownInbox[c.Key()] = c.ID
			if c.URL != "" {
				knownInbox[c.Platform+":url:"+c.URL] = c.ID
			}
		}
		net := netx.New()
		enricher := &enrich.Enricher{
			Client:          net,
			Store:           a.Store,
			BlueskyBase:     "https://public.api.bsky.app",
			KnownCanonical:  knownCanonical,
			KnownInbox:      knownInbox,
			YouTubeResolver: a.Engine.Resolve,
		}
		rep, err := enricher.RunBatch(ctx, cands, *limit, *offset)
		if err != nil {
			return err
		}
		if *syncSheet && a.Sheets != nil {
			updatedCands, uerr := a.Store.Candidates()
			if uerr == nil {
				repUpdates, serr := a.Sheets.Sync(ctx, updatedCands)
				if serr == nil {
					fmt.Fprintf(os.Stderr, "synced %d range updates to FINDER_INBOX\n", repUpdates)
				} else {
					fmt.Fprintf(os.Stderr, "sheet sync error: %v\n", serr)
				}
			}
		}
		return writeJSON(*out, rep)
	case "proposals":
		props, err := a.Store.RelationProposals()
		if err != nil {
			return err
		}
		return writeJSON(*out, props)
	case "verify":
		if a.Sheets == nil {
			return errors.New("sheet sync not configured")
		}
		tables, err := a.Sheets.Tables(ctx)
		if err != nil {
			return err
		}
		pCount := len(tables["PERSONAS"]) - 1
		aCount := len(tables["ACCOUNTS"]) - 1
		lCount := len(tables["ACCOUNT_LINKS"]) - 1
		iCount := len(tables["FINDER_INBOX"]) - 1
		dupGroups, _ := sheets.FindDuplicateInboxGroups(inbox)
		totalDups := 0
		for _, g := range dupGroups {
			totalDups += len(g.Losers)
		}
		cands, _ := a.Store.Candidates()
		bskyCands, _ := a.Store.CandidatesByPlatform("bluesky")
		props, _ := a.Store.RelationProposals()
		invOK := (pCount == 909 && aCount == 4735 && lCount == 2706 && len(dupGroups) == 0)
		res := map[string]any{
			"personas":                 pCount,
			"accounts":                 aCount,
			"account_links":            lCount,
			"finder_inbox":             iCount,
			"duplicate_inbox_keys":     len(dupGroups),
			"duplicate_inbox_rows":     totalDups,
			"total_candidates":         len(cands),
			"bluesky_candidates":       len(bskyCands),
			"relation_proposals_total": len(props),
			"canonical_invariant_ok":   invOK,
		}
		return writeJSON(*out, res)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
