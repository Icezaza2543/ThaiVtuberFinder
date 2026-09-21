package sheets

import (
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"net/url"
	"strings"
	"time"
)

type Proposal struct {
	CandidateID     string        `json:"candidate_id"`
	Action          string        `json:"action"`
	TargetPersonaID string        `json:"target_persona_id,omitempty"`
	PersonaName     string        `json:"persona_name,omitempty"`
	Account         model.Account `json:"account"`
	AccountType     string        `json:"account_type"`
	SourceURL       string        `json:"source_url"`
	Reviewer        string        `json:"reviewer"`
	ReviewedAt      string        `json:"reviewed_at"`
	Notes           string        `json:"notes,omitempty"`
}
type Handoff struct {
	SchemaVersion int        `json:"schema_version"`
	Producer      string     `json:"producer"`
	GeneratedAt   string     `json:"generated_at"`
	Proposals     []Proposal `json:"proposals"`
}

func Proposals(rows [][]string, personas map[string]bool) (Handoff, error) {
	doc := Handoff{SchemaVersion: 2, Producer: "ThaiVtuberFinder", GeneratedAt: time.Now().UTC().Format(time.RFC3339), Proposals: []Proposal{}}
	if e := checkInbox(rows); e != nil {
		return doc, e
	}
	seen := map[string]bool{}
	for n, row := range rows[1:] {
		r := padded(row)
		if r[11] != "verified" || r[12] == "ignore" {
			continue
		}
		fail := func(message string) (Handoff, error) { return doc, fmt.Errorf("review row %d: %s", n+2, message) }
		if r[0] == "" || seen[r[0]] {
			return fail("missing or duplicate candidate_id")
		}
		seen[r[0]] = true
		if r[16] == "" {
			return fail("reviewer is required")
		}
		if _, e := time.Parse(time.RFC3339, r[17]); e != nil {
			return fail("reviewed_at must be RFC3339")
		}
		u, e := url.Parse(r[8])
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fail("valid source_url is required")
		}
		a, e := model.Normalize(r[5])
		if e != nil || a.Platform == "youtube_video" || a.Platform != r[1] {
			return fail("canonical account URL invalid")
		}
		if r[2] == "" {
			return fail("resolve stable platform_id before exporting")
		}
		if a.PlatformID != "" && a.PlatformID != r[2] {
			return fail("platform_id contradicts URL")
		}
		if a.Platform == "youtube" && !model.YouTubeID.MatchString(r[2]) {
			return fail("invalid YouTube channel ID")
		}
		a.PlatformID = r[2]
		a.Name = r[4]
		a.Handle = r[3]
		if r[15] == "" || !strings.Contains("|persona|organization|group|secondary|archive|unknown|", "|"+r[15]+"|") {
			return fail("invalid account_type")
		}
		switch r[12] {
		case "create_persona":
			if strings.TrimSpace(r[14]) == "" || r[13] != "" {
				return fail("create_persona needs a name and an empty target ID")
			}
		case "link_persona":
			if !personas[r[13]] {
				return fail("target_persona_id does not exist in canonical PERSONAS")
			}
		case "account_only":
			if r[13] != "" {
				return fail("account_only must not link a persona")
			}
		default:
			return fail("choose create_persona, link_persona, account_only, or ignore")
		}
		if r[12] != "account_only" && (r[15] == "organization" || r[15] == "group" || r[15] == "unknown") {
			return fail("organization/group/unknown account cannot create or link a persona")
		}
		doc.Proposals = append(doc.Proposals, Proposal{r[0], r[12], r[13], r[14], a, r[15], r[8], r[16], r[17], r[18]})
	}
	return doc, nil
}

var ErrNotConfigured = errors.New("Google Sheets credentials not configured")
