package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/app"
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
		fmt.Println("ThaiVtuberFinder " + version + "\nCommands: once, worker, import, export, compare, status, doctor, demo, healthcheck\nUse: finder <command> -config config/finder.json [-file path] [-out path]\nexport creates reviewed proposals; compare needs -canonical <bootstrap.json>.")
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
	a, e := app.New(c)
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
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
