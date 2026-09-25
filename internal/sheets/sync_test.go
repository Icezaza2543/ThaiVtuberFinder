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

func TestExistingDuplicateAccountKeyDoesNotBlockSyncAndDoesNotDuplicate(t *testing.T) {
	c := candidate()
	row1 := make([]string, 20)
	row1[0] = "cand_1"
	row1[1] = c.Platform
	row1[2] = c.PlatformID
	row1[5] = c.URL
	row1[11] = "pending"
	row1[9] = "2026-09-20T00:00:00Z"

	row2 := make([]string, 20)
	row2[0] = "cand_2"
	row2[1] = c.Platform
	row2[2] = c.PlatformID
	row2[5] = c.URL
	row2[11] = "pending"
	row2[9] = "2026-09-21T00:00:00Z"

	rows := [][]string{InboxHeaders, row1, row2}
	c.ID = "cand_new"
	plan, e := PlanSync(rows, []model.Candidate{c}, nil)
	if e != nil {
		t.Fatalf("PlanSync should not fail on existing duplicate account keys: %v", e)
	}
	if len(plan) != 2 {
		t.Fatalf("expected 2 updates (A:K and T) for existing row, got %d", len(plan))
	}
	if plan[0].Range != "'FINDER_INBOX'!A2:K2" {
		t.Fatalf("expected update to row 2 ('FINDER_INBOX'!A2:K2), got %s", plan[0].Range)
	}
	if plan[0].Values[0][0] != "cand_1" {
		t.Fatalf("expected candidate_id to be preserved as cand_1, got %s", plan[0].Values[0][0])
	}

	// Now verify reviewed row is preferred over pending duplicate row
	row2[11] = "verified"
	row2[12] = "link_persona"
	rowsReviewed := [][]string{InboxHeaders, row1, row2}
	planReviewed, e := PlanSync(rowsReviewed, []model.Candidate{c}, nil)
	if e != nil {
		t.Fatalf("PlanSync failed with reviewed duplicate row: %v", e)
	}
	if len(planReviewed) != 2 {
		t.Fatalf("expected 2 updates for reviewed row, got %d", len(planReviewed))
	}
	if planReviewed[0].Range != "'FINDER_INBOX'!A3:K3" {
		t.Fatalf("expected update to reviewed row 3 ('FINDER_INBOX'!A3:K3), got %s", planReviewed[0].Range)
	}
	if planReviewed[0].Values[0][0] != "cand_2" {
		t.Fatalf("expected candidate_id to be preserved as cand_2, got %s", planReviewed[0].Values[0][0])
	}
	for _, u := range planReviewed {
		if strings.Contains(u.Range, ":S") || strings.Contains(u.Range, ":L") || strings.Contains(u.Range, "!L") {
			t.Fatalf("machine sync must never overwrite review columns L:S: %s", u.Range)
		}
	}
}
