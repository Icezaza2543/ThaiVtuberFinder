package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sqlite"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Store struct{ db *sqlite.DB }

func Open(path string) (*Store, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := sqlite.Open(path)
	if e != nil {
		return nil, e
	}
	s := &Store{db: db}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS candidates(id TEXT PRIMARY KEY, account_key TEXT NOT NULL UNIQUE, platform TEXT NOT NULL, platform_id TEXT NOT NULL, url TEXT NOT NULL, payload TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS candidate_url ON candidates(platform,url)`,
		`CREATE TABLE IF NOT EXISTS quotas(service TEXT NOT NULL, day TEXT NOT NULL, spent INTEGER NOT NULL, PRIMARY KEY(service,day))`,
		`CREATE TABLE IF NOT EXISTS source_runs(name TEXT PRIMARY KEY, checked_at TEXT NOT NULL, item_count INTEGER NOT NULL, error TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS resolver_cache(input_url TEXT PRIMARY KEY, payload TEXT NOT NULL, checked_at TEXT NOT NULL)`,
	} {
		if e = db.Exec(q); e != nil {
			db.Close()
			return nil, e
		}
	}
	if path != ":memory:" {
		_ = os.Chmod(path, 0600)
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Ping() error  { return s.db.Exec("SELECT 1") }
func newID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return "candidate_" + hex.EncodeToString(b), nil
}
func (s *Store) Upsert(a model.Account, source string, now time.Time) (model.Candidate, error) {
	var out model.Candidate
	if a.Platform == "" || a.URL == "" {
		return out, errors.New("candidate requires platform and URL")
	}
	e := s.db.Transaction(func(tx *sqlite.Tx) error {
		rows, e := tx.Query(`SELECT payload FROM candidates WHERE account_key=?`, a.Key())
		if e != nil {
			return e
		}
		if len(rows) == 0 && a.PlatformID != "" && a.InputURL != "" {
			rows, e = tx.Query(`SELECT payload FROM candidates WHERE platform=? AND platform_id='' AND url=?`, a.Platform, a.InputURL)
			if e != nil {
				return e
			}
		}
		if len(rows) > 0 {
			if e = json.Unmarshal([]byte(rows[0][0]), &out); e != nil {
				return e
			}
		} else {
			out.ID, e = newID()
			if e != nil {
				return e
			}
			out.FirstSeen = now.UTC().Format(time.RFC3339)
			out.MatchState = "NEW"
		}
		if a.Name == "" {
			a.Name = out.Name
		}
		out.Account = a
		out.SourceURL = source
		out.LastChecked = now.UTC().Format(time.RFC3339)
		out.Classification = a.ClassificationHint
		if out.Classification == "" {
			out.Classification = model.Classify(a.Description, a.Name)
		}
		out.ThaiRelevance = a.ThaiRelevanceHint
		if out.ThaiRelevance == "" {
			out.ThaiRelevance = model.ThaiSignal(a.Description + " " + a.Name)
		}
		body, e := json.Marshal(out)
		if e != nil {
			return e
		}
		return tx.Exec(`INSERT INTO candidates VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET account_key=excluded.account_key,platform=excluded.platform,platform_id=excluded.platform_id,url=excluded.url,payload=excluded.payload`, out.ID, a.Key(), a.Platform, a.PlatformID, a.URL, string(body))
	})
	return out, e
}
func (s *Store) Candidates() ([]model.Candidate, error) {
	rows, e := s.db.Query(`SELECT payload FROM candidates ORDER BY id`)
	if e != nil {
		return nil, e
	}
	out := make([]model.Candidate, 0, len(rows))
	for _, r := range rows {
		var c model.Candidate
		if e = json.Unmarshal([]byte(r[0]), &c); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Store) Spend(service, day string, units, limit int) error {
	if units < 1 || limit < 1 {
		return errors.New("invalid quota configuration")
	}
	return s.db.Transaction(func(tx *sqlite.Tx) error {
		rows, e := tx.Query(`SELECT spent FROM quotas WHERE service=? AND day=?`, service, day)
		if e != nil {
			return e
		}
		spent := 0
		if len(rows) > 0 {
			spent, e = strconv.Atoi(rows[0][0])
			if e != nil {
				return e
			}
		}
		if spent+units > limit {
			return fmt.Errorf("%s daily quota budget exhausted", service)
		}
		return tx.Exec(`INSERT INTO quotas VALUES(?,?,?) ON CONFLICT(service,day) DO UPDATE SET spent=excluded.spent`, service, day, strconv.Itoa(spent+units))
	})
}
func (s *Store) RunStatus(name string, n int, runErr error) error {
	message := ""
	if runErr != nil {
		message = runErr.Error()
	}
	return s.db.Exec(`INSERT INTO source_runs VALUES(?,?,?,?) ON CONFLICT(name) DO UPDATE SET checked_at=excluded.checked_at,item_count=excluded.item_count,error=excluded.error`, name, time.Now().UTC().Format(time.RFC3339), strconv.Itoa(n), message)
}
func (s *Store) Status() (map[string]any, error) {
	rows, e := s.db.Query(`SELECT COUNT(*) FROM candidates`)
	if e != nil {
		return nil, e
	}
	n, _ := strconv.Atoi(rows[0][0])
	runs, e := s.db.Query(`SELECT name,checked_at,item_count,error FROM source_runs ORDER BY name`)
	if e != nil {
		return nil, e
	}
	return map[string]any{"candidates": n, "source_runs": runs}, nil
}
func (s *Store) Cached(raw string, ttl time.Duration) (model.Account, bool, error) {
	var a model.Account
	rows, e := s.db.Query(`SELECT payload,checked_at FROM resolver_cache WHERE input_url=?`, raw)
	if e != nil || len(rows) == 0 {
		return a, false, e
	}
	t, e := time.Parse(time.RFC3339, rows[0][1])
	if e != nil || time.Since(t) > ttl {
		return a, false, e
	}
	e = json.Unmarshal([]byte(rows[0][0]), &a)
	return a, e == nil, e
}
func (s *Store) Cache(raw string, a model.Account) error {
	if a.ClassificationHint == "" {
		a.ClassificationHint = model.Classify(a.Description, a.Name)
	}
	if a.ThaiRelevanceHint == "" {
		a.ThaiRelevanceHint = model.ThaiSignal(a.Description + " " + a.Name)
	}
	b, e := json.Marshal(a)
	if e != nil {
		return e
	}
	return s.db.Exec(`INSERT INTO resolver_cache VALUES(?,?,?) ON CONFLICT(input_url) DO UPDATE SET payload=excluded.payload,checked_at=excluded.checked_at`, raw, string(b), time.Now().UTC().Format(time.RFC3339))
}
