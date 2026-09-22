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
		`CREATE TABLE IF NOT EXISTS candidate_sources(candidate_id TEXT NOT NULL, source_name TEXT NOT NULL, source_url TEXT NOT NULL, discovered_at TEXT NOT NULL, PRIMARY KEY(candidate_id, source_url))`,
		`CREATE INDEX IF NOT EXISTS idx_candidate_sources_id ON candidate_sources(candidate_id)`,
		`CREATE TABLE IF NOT EXISTS account_relation_proposals(proposal_id TEXT PRIMARY KEY, from_candidate_id TEXT NOT NULL, from_platform TEXT NOT NULL, from_platform_id TEXT NOT NULL, to_platform TEXT NOT NULL, to_platform_id TEXT NOT NULL, to_url TEXT NOT NULL, evidence_url TEXT NOT NULL, evidence_type TEXT NOT NULL, confidence TEXT NOT NULL, created_at TEXT NOT NULL, review_status TEXT NOT NULL, UNIQUE(from_candidate_id, to_platform, to_url))`,
		`CREATE INDEX IF NOT EXISTS idx_rel_from_cand ON account_relation_proposals(from_candidate_id)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_to_url ON account_relation_proposals(to_url)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_to_plat ON account_relation_proposals(to_platform, to_platform_id)`,
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
		if out.SourceURL == "" {
			out.SourceURL = source
		}
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
		if e = tx.Exec(`INSERT INTO candidates VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET account_key=excluded.account_key,platform=excluded.platform,platform_id=excluded.platform_id,url=excluded.url,payload=excluded.payload`, out.ID, a.Key(), a.Platform, a.PlatformID, a.URL, string(body)); e != nil {
			return e
		}
		if source != "" {
			return tx.Exec(`INSERT INTO candidate_sources(candidate_id, source_name, source_url, discovered_at)
VALUES(?,?,?,?)
ON CONFLICT(candidate_id, source_url) DO UPDATE SET discovered_at=excluded.discovered_at`,
				out.ID, source, source, now.UTC().Format(time.RFC3339))
		}
		return nil
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
func (s *Store) CandidatesByPlatform(platform string) ([]model.Candidate, error) {
	rows, e := s.db.Query(`SELECT payload FROM candidates WHERE platform=? ORDER BY id`, platform)
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
func (s *Store) AddRelationProposal(p model.RelationProposal) error {
	if p.ProposalID == "" {
		id, err := newID()
		if err != nil {
			return err
		}
		p.ProposalID = "prop_" + strings.TrimPrefix(id, "candidate_")
	}
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if p.ReviewStatus == "" {
		p.ReviewStatus = "pending_review"
	}
	return s.db.Transaction(func(tx *sqlite.Tx) error {
		return tx.Exec(`INSERT INTO account_relation_proposals(proposal_id, from_candidate_id, from_platform, from_platform_id, to_platform, to_platform_id, to_url, evidence_url, evidence_type, confidence, created_at, review_status)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(from_candidate_id, to_platform, to_url) DO UPDATE SET
to_platform_id=excluded.to_platform_id,
evidence_url=excluded.evidence_url,
confidence=excluded.confidence,
review_status=excluded.review_status`,
			p.ProposalID, p.FromCandidateID, p.FromPlatform, p.FromPlatformID, p.ToPlatform, p.ToPlatformID, p.ToURL, p.EvidenceURL, p.EvidenceType, p.Confidence, p.CreatedAt, p.ReviewStatus)
	})
}
func (s *Store) RelationProposals() ([]model.RelationProposal, error) {
	rows, err := s.db.Query(`SELECT proposal_id, from_candidate_id, from_platform, from_platform_id, to_platform, to_platform_id, to_url, evidence_url, evidence_type, confidence, created_at, review_status FROM account_relation_proposals ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	out := make([]model.RelationProposal, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.RelationProposal{
			ProposalID:      r[0],
			FromCandidateID: r[1],
			FromPlatform:    r[2],
			FromPlatformID:  r[3],
			ToPlatform:      r[4],
			ToPlatformID:    r[5],
			ToURL:           r[6],
			EvidenceURL:     r[7],
			EvidenceType:    r[8],
			Confidence:      r[9],
			CreatedAt:       r[10],
			ReviewStatus:    r[11],
		})
	}
	return out, nil
}
func (s *Store) CandidateSources(candidateID string) ([]model.CandidateSource, error) {
	rows, err := s.db.Query(`SELECT candidate_id, source_name, source_url, discovered_at FROM candidate_sources WHERE candidate_id=? ORDER BY discovered_at ASC`, candidateID)
	if err != nil {
		return nil, err
	}
	out := make([]model.CandidateSource, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.CandidateSource{
			CandidateID:  r[0],
			SourceName:   r[1],
			SourceURL:    r[2],
			DiscoveredAt: r[3],
		})
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
	propRows, _ := s.db.Query(`SELECT COUNT(*) FROM account_relation_proposals`)
	propCount := 0
	if len(propRows) > 0 {
		propCount, _ = strconv.Atoi(propRows[0][0])
	}
	runs, e := s.db.Query(`SELECT name,checked_at,item_count,error FROM source_runs ORDER BY name`)
	if e != nil {
		return nil, e
	}
	return map[string]any{"candidates": n, "relation_proposals": propCount, "source_runs": runs}, nil
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
