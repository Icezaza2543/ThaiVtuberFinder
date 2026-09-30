package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sources"
)

// Validate the real enabled files offline, without API resolution or sheet writes.
func TestXProfileWebsiteSeedsParseCompletely(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "config", "finder.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]int{
		"x-profile-websites-2026-09-30":          437,
		"x-profile-websites-resolved-2026-10-01": 11,
	}
	engine := sources.Engine{}
	for _, s := range c.Sources {
		if s.Enabled && strings.Contains(s.Path, "x-profile-websites-2026-09-30-unresolved") {
			t.Fatal("unresolved evidence must not be an enabled source")
		}
		want, ok := expected[s.Name]
		if !ok {
			continue
		}
		if !s.Enabled || s.Kind != "jsonl" {
			t.Fatalf("%s must be an enabled JSONL source", s.Name)
		}
		s.Path = filepath.Join("..", "..", s.Path)
		leads, err := engine.Discover(context.Background(), s)
		if err != nil || len(leads) != want {
			t.Fatalf("%s: got %d leads, want %d; error=%v", s.Name, len(leads), want, err)
		}
		delete(expected, s.Name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing configured X-profile sources: %v", expected)
	}
}
