package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sources"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Database           string           `json:"database"`
	Interval           string           `json:"interval"`
	CycleTimeout       string           `json:"cycle_timeout"`
	Workers            int              `json:"workers"`
	ResolveYouTube     bool             `json:"resolve_youtube"`
	YouTubeDailyBudget int              `json:"youtube_daily_budget"`
	SheetSync          bool             `json:"sheet_sync"`
	SpreadsheetID      string           `json:"spreadsheet_id"`
	CanonicalFile      string           `json:"canonical_file"`
	HealthAddress      string           `json:"health_address"`
	Sources            []sources.Config `json:"sources"`
}

func Defaults() Config {
	return Config{Database: "data/finder.sqlite", Interval: "6h", CycleTimeout: "15m", Workers: 4, ResolveYouTube: true, YouTubeDailyBudget: 500, HealthAddress: "127.0.0.1:8080"}
}
func Load(path string) (Config, error) {
	c := Defaults()
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	if v := os.Getenv("FINDER_DB"); v != "" {
		c.Database = v
	}
	if v := os.Getenv("GOOGLE_SHEET_ID"); v != "" {
		c.SpreadsheetID = v
	}
	if v := os.Getenv("SHEETS_SYNC"); v != "" {
		c.SheetSync, e = strconv.ParseBool(v)
		if e != nil {
			return c, errors.New("SHEETS_SYNC must be true or false")
		}
	}
	if v := os.Getenv("HEALTH_ADDRESS"); v != "" {
		c.HealthAddress = v
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.Database == "" || c.Workers < 1 || c.Workers > 16 {
		return errors.New("database required; workers must be 1..16")
	}
	for n, v := range map[string]string{"interval": c.Interval, "cycle_timeout": c.CycleTimeout} {
		d, e := time.ParseDuration(v)
		if e != nil || d <= 0 {
			return fmt.Errorf("%s must be a positive duration", n)
		}
	}
	if c.YouTubeDailyBudget < 1 || c.YouTubeDailyBudget > 10000 {
		return errors.New("youtube_daily_budget must be 1..10000; coordinate with other workers")
	}
	if c.SheetSync && c.SpreadsheetID == "" {
		return errors.New("GOOGLE_SHEET_ID required when SHEETS_SYNC=true")
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		if !s.Enabled {
			continue
		}
		if s.Name == "" || seen[s.Name] {
			return errors.New("enabled source names must be nonempty and unique")
		}
		seen[s.Name] = true
		if s.MaxItems < 0 || s.MaxItems > 20000 || s.MaxPages < 0 || s.MaxPages > 100 {
			return errors.New("source limit out of bounds")
		}
		switch s.Kind {
		case "kerlos", "html", "jsonl", "csv", "json", "bluesky_list", "bluesky_starterpack", "youtube_search":
		default:
			return fmt.Errorf("unsupported source: %s", s.Kind)
		}
	}
	return nil
}

// LoadEnv reads local KEY=value settings without executing shell code or expansion.
// Process environment wins. The file is optional and must never be committed.
func LoadEnv(path string) error {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1<<20)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return errors.New("invalid .env line")
		}
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, set := os.LookupEnv(k); !set {
			if e = os.Setenv(k, v); e != nil {
				return e
			}
		}
	}
	return s.Err()
}
