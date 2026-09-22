package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sheets"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sources"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/store"
	"net/http"
	"os"
	"sync"
	"time"
)

type SourceResult struct {
	Name          string `json:"name"`
	Found         int    `json:"found"`
	Stored        int    `json:"stored"`
	ResolveErrors int    `json:"resolve_errors"`
	Error         string `json:"error,omitempty"`
}
type Report struct {
	StartedAt       string         `json:"started_at"`
	FinishedAt      string         `json:"finished_at"`
	Stored          int            `json:"stored_observations"`
	TotalCandidates int            `json:"total_candidates"`
	FailedSources   int            `json:"failed_sources"`
	SheetUpdates    int            `json:"sheet_range_updates"`
	Sources         []SourceResult `json:"sources"`
	Error           string         `json:"error,omitempty"`
}
type App struct {
	Config Config
	Store  *store.Store
	Engine *sources.Engine
	Sheets *sheets.Client
	lock   *Lock
	mu     sync.RWMutex
	last   Report
}

func New(c Config) (*App, error) {
	return NewWithOptions(c, true)
}

func NewUnlocked(c Config) (*App, error) {
	return NewWithOptions(c, false)
}

func NewWithOptions(c Config, exclusiveLock bool) (*App, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	var lock *Lock
	var e error
	if exclusiveLock {
		lock, e = AcquireLock(c.Database + ".lock")
		if e != nil {
			return nil, e
		}
	}
	db, e := store.Open(c.Database)
	if e != nil {
		if lock != nil {
			lock.Close()
		}
		return nil, e
	}
	net := netx.New()
	a := &App{Config: c, Store: db, lock: lock, Engine: &sources.Engine{Client: net, Store: db, APIKey: os.Getenv("YOUTUBE_API_KEY"), DailyBudget: c.YouTubeDailyBudget}}
	if c.SheetSync {
		tokens, e := sheets.Credentials(net)
		if e != nil {
			a.Close()
			return nil, e
		}
		a.Sheets = &sheets.Client{HTTP: net, Tokens: tokens, SpreadsheetID: c.SpreadsheetID}
	}
	return a, nil
}
func (a *App) Close() {
	if a.Store != nil {
		a.Store.Close()
	}
	if a.lock != nil {
		a.lock.Close()
	}
}
func (a *App) Once(ctx context.Context) (report Report, err error) {
	timeout, _ := time.ParseDuration(a.Config.CycleTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	report.StartedAt = time.Now().UTC().Format(time.RFC3339)
	report.Sources = []SourceResult{}
	defer func() {
		report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			report.Error = err.Error()
		}
		a.mu.Lock()
		a.last = report
		a.mu.Unlock()
	}()
	jobs := make(chan sources.Config)
	results := make(chan SourceResult)
	var workers sync.WaitGroup
	for n := 0; n < a.Config.Workers; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for s := range jobs {
				r := SourceResult{Name: s.Name}
				leads, discoveryErr := a.Engine.Discover(ctx, s)
				r.Found = len(leads)
				for _, lead := range leads {
					if ctx.Err() != nil {
						discoveryErr = ctx.Err()
						break
					}
					account := lead.Account
					if a.Config.ResolveYouTube && a.Engine.APIKey != "" && (account.Platform == "youtube_video" || (account.Platform == "youtube" && account.PlatformID == "")) {
						resolved, e := a.Engine.Resolve(ctx, account)
						if e != nil {
							r.ResolveErrors++
						} else {
							account = resolved
						}
					}
					if _, e := a.Store.Upsert(account, lead.SourceURL, time.Now()); e != nil {
						discoveryErr = e
						break
					}
					r.Stored++
				}
				if discoveryErr != nil {
					r.Error = discoveryErr.Error()
				}
				if r.ResolveErrors > 0 && r.Error == "" {
					r.Error = fmt.Sprintf("%d accounts still require YouTube resolution", r.ResolveErrors)
				}
				var runErr error
				if r.Error != "" {
					runErr = errors.New(r.Error)
				}
				if e := a.Store.RunStatus(s.Name, r.Stored, runErr); e != nil {
					r.Error = "cannot persist source run status"
				}
				results <- r
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, s := range a.Config.Sources {
			if s.Enabled {
				select {
				case jobs <- s:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() { workers.Wait(); close(results) }()
	for r := range results {
		report.Stored += r.Stored
		if r.Error != "" {
			report.FailedSources++
		}
		report.Sources = append(report.Sources, r)
	}
	candidates, e := a.Store.Candidates()
	if e != nil {
		return report, e
	}
	report.TotalCandidates = len(candidates)
	if a.Config.SheetSync {
		if a.Sheets == nil {
			return report, sheets.ErrNotConfigured
		}
		report.SheetUpdates, e = a.Sheets.Sync(ctx, candidates)
		if e != nil {
			return report, e
		}
	}
	if ctx.Err() != nil {
		return report, ctx.Err()
	}
	if report.FailedSources > 0 {
		return report, errors.New("one or more sources were incomplete; valid candidates were retained")
	}
	return report, nil
}
func (a *App) HealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if e := a.Store.Ping(); e != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		last := a.last
		a.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if last.FinishedAt == "" || last.Error != "" {
			w.WriteHeader(503)
		}
		json.NewEncoder(w).Encode(map[string]any{"last_cycle": last.FinishedAt, "candidates": last.TotalCandidates, "failed_sources": last.FailedSources})
	})
	return mux
}
func (a *App) Run(ctx context.Context, emit func(Report)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := &http.Server{Addr: a.Config.HealthAddress, Handler: a.HealthHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		e := srv.ListenAndServe()
		if e != nil && e != http.ErrServerClosed {
			serverErr <- e
			cancel()
		}
	}()
	defer func() {
		stop, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		srv.Shutdown(stop)
	}()
	interval, _ := time.ParseDuration(a.Config.Interval)
	for {
		r, e := a.Once(ctx)
		emit(r)
		if ctx.Err() != nil {
			select {
			case err := <-serverErr:
				return err
			default:
				return nil
			}
		}
		wait := interval
		if e != nil && wait > 15*time.Minute {
			wait = 15 * time.Minute
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			select {
			case err := <-serverErr:
				return err
			default:
				return nil
			}
		case <-timer.C:
		}
	}
}
func CandidatesWithMatches(candidates []model.Candidate, known map[string]string) []model.Candidate {
	out := append([]model.Candidate{}, candidates...)
	for i := range out {
		match := known[out[i].Key()]
		if match == "" {
			match = known[out[i].Platform+":url:"+out[i].URL]
		}
		if match == "" {
			match = "NEW"
		}
		out[i].MatchState = match
	}
	return out
}
