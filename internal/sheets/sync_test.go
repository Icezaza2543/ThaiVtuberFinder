package sheets

import (
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"strings"
	"testing"
)

func candidate() model.Candidate {
	return model.Candidate{ID: "candidate_test", Account: model.Account{Platform: "youtube", PlatformID: "UCaaaaaaaaaaaaaaaaaaaaaa", URL: "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa", Name: "Example VTuberTH"}, SourceURL: "https://example.org/roster", FirstSeen: "2026-09-22T00:00:00Z", LastChecked: "2026-09-22T00:00:00Z", Classification: "VTUBER_SIGNAL", ThaiRelevance: "thai_signal"}
}
func TestPlanPreservesReviewColumns(t *testing.T) {
	c := candidate()
	row := make([]string, 20)
	row[0] = c.ID
	row[1] = c.Platform
	row[2] = c.PlatformID
	row[5] = c.URL
	row[11] = "verified"
	row[12] = "link_persona"
	row[13] = "persona_one"
	row[16] = "owner"
	row[17] = "2026-09-22T00:00:00Z"
	rows := [][]string{InboxHeaders, row}
	plan, e := PlanSync(rows, []model.Candidate{c}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 2 {
		t.Fatal(plan)
	}
	if plan[0].Range != "'FINDER_INBOX'!A2:K2" || plan[1].Range != "'FINDER_INBOX'!T2" {
		t.Fatal("review columns overwritten", plan)
	}
}
func TestPlanNewRowsAndLostLocalDatabase(t *testing.T) {
	c := candidate()
	plan, e := PlanSync([][]string{InboxHeaders}, []model.Candidate{c}, nil)
	if e != nil || len(plan) != 1 || plan[0].Range != "'FINDER_INBOX'!A2:T2" {
		t.Fatal(e, plan)
	}
	row := make([]string, 20)
	for i, v := range plan[0].Values[0] {
		row[i] = v
	}
	rows := [][]string{InboxHeaders, row}
	c.ID = "candidate_after_local_reset"
	plan, e = PlanSync(rows, []model.Candidate{c}, nil)
	if e != nil || len(plan) != 2 || plan[0].Values[0][0] != row[0] {
		t.Fatal("did not reconcile stable ID", e, plan)
	}
}
func TestDuplicateRowsFailClosed(t *testing.T) {
	r := make([]string, 20)
	r[0] = "same"
	_, e := PlanSync([][]string{InboxHeaders, r, r}, nil, nil)
	if e == nil {
		t.Fatal("duplicate silently accepted")
	}
}
func TestKnownLinkedSkippedButKnownUnlinkedQueued(t *testing.T) {
	c := candidate()
	p, e := PlanSync([][]string{InboxHeaders}, []model.Candidate{c}, map[string]string{c.Key(): "KNOWN_LINKED"})
	if e != nil || len(p) != 0 {
		t.Fatal(e, p)
	}
	p, e = PlanSync([][]string{InboxHeaders}, []model.Candidate{c}, map[string]string{c.Key(): "KNOWN_ACCOUNT"})
	if e != nil || len(p) != 1 || !strings.Contains(p[0].Values[0][19], "known_account") {
		t.Fatal(e, p)
	}
}
func TestHeaderDriftRefused(t *testing.T) {
	h := append([]string{}, InboxHeaders...)
	h[3] = "bad"
	if _, e := PlanSync([][]string{h}, nil, nil); e == nil {
		t.Fatal("schema drift accepted")
	}
}
func TestProposalRequiresHumanApprovalAndValidTarget(t *testing.T) {
	c := candidate()
	p, _ := PlanSync([][]string{InboxHeaders}, []model.Candidate{c}, nil)
	r := append([]string{}, p[0].Values[0]...)
	doc, e := Proposals([][]string{InboxHeaders, r}, map[string]bool{})
	if e != nil || len(doc.Proposals) != 0 {
		t.Fatal(e)
	}
	r[11] = "verified"
	r[12] = "link_persona"
	r[13] = "persona_x"
	r[15] = "persona"
	r[16] = "owner"
	r[17] = "2026-09-22T00:00:00Z"
	if _, e = Proposals([][]string{InboxHeaders, r}, map[string]bool{}); e == nil {
		t.Fatal("unknown target accepted")
	}
	doc, e = Proposals([][]string{InboxHeaders, r}, map[string]bool{"persona_x": true})
	if e != nil || len(doc.Proposals) != 1 {
		t.Fatal(e)
	}
	r[15] = "organization"
	if _, e = Proposals([][]string{InboxHeaders, r}, map[string]bool{"persona_x": true}); e == nil {
		t.Fatal("organization promoted as persona")
	}
}
func TestSyncKeepsOriginalFirstSeenAfterLocalReset(t *testing.T) {
	c := candidate()
	p, _ := PlanSync([][]string{InboxHeaders}, []model.Candidate{c}, nil)
	r := append([]string{}, p[0].Values[0]...)
	c.FirstSeen = "2026-10-01T00:00:00Z"
	p, e := PlanSync([][]string{InboxHeaders, r}, []model.Candidate{c}, nil)
	if e != nil || p[0].Values[0][9] != r[9] {
		t.Fatal("lost original first_seen", e)
	}
}
