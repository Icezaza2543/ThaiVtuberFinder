// Package sheets writes the Finder inbox without touching curator-owned columns.
package sheets

import (
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"reflect"
	"sort"
	"time"
)

var InboxHeaders = []string{"candidate_id", "platform", "platform_id", "handle", "display_name", "canonical_url", "classification", "thai_relevance", "source_url", "first_seen", "last_checked_at", "review_status", "action", "target_persona_id", "persona_name", "account_type", "reviewer", "reviewed_at", "notes", "sync_state"}

type Update struct {
	Range  string     `json:"range"`
	Values [][]string `json:"values"`
}

func checkInbox(rows [][]string) error {
	if len(rows) == 0 || !reflect.DeepEqual(rows[0], InboxHeaders) {
		return errors.New("FINDER_INBOX schema differs from v2; refusing write")
	}
	return nil
}
func padded(row []string) []string { v := make([]string, 20); copy(v, row); return v }
func rowAccount(row []string) model.Account {
	r := padded(row)
	return model.Account{Platform: r[1], PlatformID: r[2], Handle: r[3], Name: r[4], URL: r[5]}
}
func PlanSync(existing [][]string, candidates []model.Candidate, known map[string]string) ([]Update, error) {
	if e := checkInbox(existing); e != nil {
		return nil, e
	}
	byID := map[string]int{}
	byKey := map[string]int{}
	for i, r := range existing[1:] {
		row := padded(r)
		if row[0] == "" {
			for _, v := range row {
				if v != "" {
					return nil, fmt.Errorf("inbox row %d lacks candidate_id", i+2)
				}
			}
			continue
		}
		if _, ok := byID[row[0]]; ok {
			return nil, errors.New("duplicate candidate_id in inbox")
		}
		byID[row[0]] = i + 1
		a := rowAccount(r)
		if a.Platform != "" && a.URL != "" {
			k := a.Key()
			if prevIdx, ok := byKey[k]; !ok {
				byKey[k] = i + 1
			} else {
				prevRow := padded(existing[prevIdx])
				if prevRow[11] == "pending" && row[11] != "pending" && row[11] != "" {
					byKey[k] = i + 1
				}
			}
		}
	}
	incoming := append([]model.Candidate{}, candidates...)
	sort.Slice(incoming, func(i, j int) bool { return incoming[i].ID < incoming[j].ID })
	seen := map[string]bool{}
	out := []Update{}
	next := len(existing) + 1
	for _, c := range incoming {
		if c.ID == "" || c.Platform == "" || c.URL == "" {
			return nil, errors.New("invalid candidate")
		}
		if seen[c.Key()] {
			return nil, errors.New("duplicate incoming account")
		}
		seen[c.Key()] = true
		idx, found := byID[c.ID]
		if !found {
			idx, found = byKey[c.Key()]
		}
		match := known[c.Key()]
		if match == "" {
			match = known[c.Platform+":url:"+c.URL]
		}
		if match == "KNOWN_LINKED" && !found {
			continue
		}
		state := "pending_review"
		if c.PlatformID == "" || c.Platform == "youtube_video" {
			state = "unresolved_platform_id"
		}
		if match == "KNOWN_ACCOUNT" {
			state = "known_account_needs_review"
		}
		if match == "KNOWN_LINKED" {
			state = "known_linked"
		}
		id := c.ID
		if found {
			id = existing[idx][0]
			old := field(existing[idx], 9)
			if old != "" {
				previous, pe := time.Parse(time.RFC3339, old)
				current, ce := time.Parse(time.RFC3339, c.FirstSeen)
				if pe != nil || ce != nil || !current.Before(previous) {
					c.FirstSeen = old
				}
			}
		}
		machine := []string{id, c.Platform, c.PlatformID, c.Handle, c.Name, c.URL, c.Classification, c.ThaiRelevance, c.SourceURL, c.FirstSeen, c.LastChecked}
		if found {
			out = append(out, Update{fmt.Sprintf("'FINDER_INBOX'!A%d:K%d", idx+1, idx+1), [][]string{machine}}, Update{fmt.Sprintf("'FINDER_INBOX'!T%d", idx+1), [][]string{{state}}})
		} else {
			typ := "unknown"
			if c.Classification == "ORGANIZATION_SIGNAL" {
				typ = "organization"
			}
			row := append(machine, "pending", "", "", c.Name, typ, "", "", "", state)
			out = append(out, Update{fmt.Sprintf("'FINDER_INBOX'!A%d:T%d", next, next), [][]string{row}})
			next++
		}
	}
	return out, nil
}

// CanonicalKeys distinguishes an existing account from a reviewed persona linkage.
// An existing but unlinked account is kept in the review queue, not silently dropped.
func CanonicalKeys(accounts, links, personas [][]string) (map[string]string, map[string]bool, error) {
	known := map[string]string{}
	pids := map[string]bool{}
	verified := map[string]bool{}
	linked := map[string]bool{}
	pIndex, e := header(personas, "persona_id", "review_status")
	if e != nil {
		return nil, nil, e
	}
	for _, r := range personas[1:] {
		id := field(r, pIndex["persona_id"])
		if id != "" {
			pids[id] = true
			verified[id] = field(r, pIndex["review_status"]) == "verified"
		}
	}
	lIndex, e := header(links, "account_id", "persona_id", "review_status", "valid_to")
	if e != nil {
		return nil, nil, e
	}
	for _, r := range links[1:] {
		if field(r, lIndex["review_status"]) == "verified" && verified[field(r, lIndex["persona_id"])] && field(r, lIndex["valid_to"]) == "" {
			linked[field(r, lIndex["account_id"])] = true
		}
	}
	aIndex, e := header(accounts, "account_id", "platform", "platform_id", "canonical_url")
	if e != nil {
		return nil, nil, e
	}
	for _, r := range accounts[1:] {
		id := field(r, aIndex["account_id"])
		if id == "" {
			continue
		}
		a := model.Account{Platform: field(r, aIndex["platform"]), PlatformID: field(r, aIndex["platform_id"]), URL: field(r, aIndex["canonical_url"])}
		state := "KNOWN_ACCOUNT"
		if linked[id] {
			state = "KNOWN_LINKED"
		}
		known[a.Key()] = state
		if a.URL != "" {
			known[a.Platform+":url:"+a.URL] = state
		}
	}
	return known, pids, nil
}
func header(rows [][]string, required ...string) (map[string]int, error) {
	if len(rows) == 0 {
		return nil, errors.New("canonical sheet header missing")
	}
	m := map[string]int{}
	for i, h := range rows[0] {
		m[h] = i
	}
	for _, h := range required {
		if _, ok := m[h]; !ok {
			return nil, fmt.Errorf("canonical column missing: %s", h)
		}
	}
	return m, nil
}
func field(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}
