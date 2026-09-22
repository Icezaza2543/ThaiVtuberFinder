package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
)

type mockCandidateStore struct {
	mu         sync.Mutex
	candidates []model.Candidate
	proposals  []model.RelationProposal
}

func (m *mockCandidateStore) Upsert(a model.Account, source string, now time.Time) (model.Candidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := model.Candidate{
		ID:             "cand_mock_new",
		Account:        a,
		SourceURL:      source,
		FirstSeen:      now.Format(time.RFC3339),
		LastChecked:    now.Format(time.RFC3339),
		Classification: a.ClassificationHint,
		ThaiRelevance:  a.ThaiRelevanceHint,
	}
	m.candidates = append(m.candidates, c)
	return c, nil
}

func (m *mockCandidateStore) AddRelationProposal(p model.RelationProposal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proposals = append(m.proposals, p)
	return nil
}

func TestExtractLinks(t *testing.T) {
	text := `วีนัส 🌠 | Papa: @deejiart.bsky.social
YT : https://www.youtube.com/@venuslpr.
Twitch : twitch.tv/venuslapirus!
carrd: https://darchellevtuber.carrd.co/
tiktok: https://www.tiktok.com/@jina_vr?lang=th
twitter: (x.com/Alvazerius), check it out!
discord: https://discord.gg/dJSTYnFfSU✨มอบแฟนอาร์ต
bsky: https://bsky.app/profile/venuslapis.bsky.social`

	links := ExtractLinks(text)
	if len(links) != 6 {
		t.Fatalf("expected 6 links, got %d: %+v", len(links), links)
	}

	expected := []string{
		"https://www.youtube.com/@venuslpr",
		"https://twitch.tv/venuslapirus",
		"https://darchellevtuber.carrd.co",
		"https://www.tiktok.com/@jina_vr?lang=th",
		"https://x.com/Alvazerius",
		"https://discord.gg/dJSTYnFfSU",
	}

	for i, exp := range expected {
		if links[i] != exp {
			t.Errorf("link[%d]: expected %s, got %s", i, exp, links[i])
		}
	}
}

func TestNormalizeLink(t *testing.T) {
	// YouTube
	acc, conf, err := NormalizeLink("https://www.youtube.com/@venuslpr")
	if err != nil || acc.Platform != "youtube" || conf != "high" {
		t.Fatalf("expected high confidence youtube, got: %+v, conf=%s, err=%v", acc, conf, err)
	}

	// Twitch
	acc, conf, err = NormalizeLink("https://twitch.tv/venuslapirus")
	if err != nil || acc.Platform != "twitch" || conf != "high" {
		t.Fatalf("expected high confidence twitch, got: %+v, conf=%s, err=%v", acc, conf, err)
	}

	// Carrd
	acc, conf, err = NormalizeLink("https://darchellevtuber.carrd.co/")
	if err != nil || acc.Platform != "website" || conf != "medium" {
		t.Fatalf("expected medium confidence website, got: %+v, conf=%s, err=%v", acc, conf, err)
	}
}

func TestRunBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"profiles": [
				{
					"did": "did:plc:creator1",
					"handle": "creator1.bsky.social",
					"displayName": "Creator One",
					"description": "YT: https://youtube.com/@creatorone Twitch: twitch.tv/creatorone"
				}
			]
		}`))
	}))
	defer srv.Close()

	store := &mockCandidateStore{}
	netClient := &netx.Client{HTTP: srv.Client()}
	enricher := &Enricher{
		Client:      netClient,
		Store:       store,
		BlueskyBase: srv.URL,
		YouTubeResolver: func(ctx context.Context, a model.Account) (model.Account, error) {
			a.PlatformID = "UC1111111111111111111111"
			a.URL = "https://www.youtube.com/channel/UC1111111111111111111111"
			return a, nil
		},
	}

	candidates := []model.Candidate{
		{
			ID: "cand_1",
			Account: model.Account{
				Platform:   "bluesky",
				PlatformID: "did:plc:creator1",
				Handle:     "creator1.bsky.social",
				URL:        "https://bsky.app/profile/did:plc:creator1",
			},
		},
	}

	rep, err := enricher.RunBatch(context.Background(), candidates, 10, 0)
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}

	if rep.ProfilesScanned != 1 || rep.ProfilesWithLinks != 1 {
		t.Fatalf("unexpected report metrics: %+v", rep)
	}

	if rep.YouTubeLinksFound != 1 || rep.TwitchLinksFound != 1 {
		t.Fatalf("expected 1 youtube and 1 twitch link, got: %+v", rep)
	}

	if rep.StableIDsResolved < 2 {
		t.Fatalf("expected at least 2 stable IDs resolved, got %d", rep.StableIDsResolved)
	}

	if rep.RelationProposalsCount != 2 {
		t.Fatalf("expected 2 relation proposals, got %d", rep.RelationProposalsCount)
	}

	// Verify proposal details
	p1 := rep.Proposals[0]
	if p1.FromPlatform != "bluesky" || p1.FromPlatformID != "did:plc:creator1" || p1.ToPlatform != "youtube" || p1.ToPlatformID != "UC1111111111111111111111" {
		t.Errorf("proposal 1 unexpected: %+v", p1)
	}

	p2 := rep.Proposals[1]
	if p2.FromPlatform != "bluesky" || p2.ToPlatform != "twitch" || p2.ToPlatformID != "creatorone" {
		t.Errorf("proposal 2 unexpected: %+v", p2)
	}
}
