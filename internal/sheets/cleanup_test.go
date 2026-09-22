package sheets

import (
	"testing"
)

func TestFindDuplicateInboxGroupsAndPriorities(t *testing.T) {
	row1 := make([]string, 20)
	row1[0] = "cand_loser_1"
	row1[1] = "twitch"
	row1[3] = "foo"
	row1[5] = "https://twitch.tv/foo"
	row1[9] = "2026-09-20T12:00:00Z"
	row1[11] = "pending"

	row2 := make([]string, 20)
	row2[0] = "cand_survivor_verified"
	row2[1] = "twitch"
	row2[3] = "foo"
	row2[4] = "Foo Name"
	row2[5] = "https://www.twitch.tv/foo"
	row2[9] = "2026-09-21T12:00:00Z"
	row2[11] = "verified"
	row2[12] = "link_persona"
	row2[13] = "persona_xyz"
	row2[16] = "reviewer_bob"
	row2[17] = "2026-09-21T13:00:00Z"

	rows := [][]string{InboxHeaders, row1, row2}
	groups, err := FindDuplicateInboxGroups(rows)
	if err != nil {
		t.Fatalf("FindDuplicateInboxGroups error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	g := groups[0]
	// Verified row should be chosen as survivor over pending row
	if g.Survivor.Row[0] != "cand_survivor_verified" {
		t.Fatalf("expected cand_survivor_verified as survivor, got %s", g.Survivor.Row[0])
	}
	if len(g.Losers) != 1 || g.Losers[0].Row[0] != "cand_loser_1" {
		t.Fatalf("expected cand_loser_1 as loser, got %+v", g.Losers)
	}
	// Merged first_seen should be the earliest (from row1)
	if g.Merged[9] != "2026-09-20T12:00:00Z" {
		t.Fatalf("expected earliest first_seen 2026-09-20T12:00:00Z, got %s", g.Merged[9])
	}
	// Curator fields preserved from verified row
	if g.Merged[11] != "verified" || g.Merged[13] != "persona_xyz" || g.Merged[16] != "reviewer_bob" {
		t.Fatalf("curator fields were not preserved: %+v", g.Merged)
	}
	// Canonical URL normalized
	if g.Merged[5] != "https://www.twitch.tv/foo" {
		t.Fatalf("canonical URL was not normalized: %s", g.Merged[5])
	}
}

func TestFindDuplicateInboxGroupsTieBreaker(t *testing.T) {
	row1 := make([]string, 20)
	row1[0] = "cand_first"
	row1[1] = "twitch"
	row1[3] = "bar"
	row1[5] = "https://www.twitch.tv/bar"
	row1[9] = "2026-09-20T12:00:00Z"
	row1[11] = "pending"

	row2 := make([]string, 20)
	row2[0] = "cand_second"
	row2[1] = "twitch"
	row2[3] = "bar"
	row2[5] = "https://twitch.tv/bar"
	row2[9] = "2026-09-20T12:00:00Z"
	row2[11] = "pending"

	rows := [][]string{InboxHeaders, row1, row2}
	groups, err := FindDuplicateInboxGroups(rows)
	if err != nil {
		t.Fatalf("FindDuplicateInboxGroups error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}
	// When both pending, earlier row is survivor
	if groups[0].Survivor.Row[0] != "cand_first" {
		t.Fatalf("expected cand_first to win tie-breaker, got %s", groups[0].Survivor.Row[0])
	}
}
