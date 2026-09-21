package store

import (
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"path/filepath"
	"testing"
	"time"
)

func TestStableDedupePersists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db.sqlite")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	a := model.Account{Platform: "youtube", PlatformID: "UCaaaaaaaaaaaaaaaaaaaaaa", URL: "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa", Name: "First"}
	first, e := s.Upsert(a, "https://roster.example/a", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	a.Name = "Renamed"
	second, e := s.Upsert(a, "https://roster.example/b", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if first.ID != second.ID {
		t.Fatal("changed ID")
	}
	s.Close()
	s, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	rows, e := s.Candidates()
	if e != nil || len(rows) != 1 || rows[0].Name != "Renamed" {
		t.Fatalf("%v %+v", e, rows)
	}
}
func TestUnresolvedUpgrade(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "db"))
	defer s.Close()
	a, _ := model.Normalize("https://youtube.com/@example")
	first, e := s.Upsert(a, "https://example.org", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	a.InputURL = a.URL
	a.PlatformID = "UCaaaaaaaaaaaaaaaaaaaaaa"
	a.URL = "https://www.youtube.com/channel/" + a.PlatformID
	next, e := s.Upsert(a, "https://example.org", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if first.ID != next.ID {
		t.Fatal("unresolved upgrade duplicated")
	}
}
func TestQuotaPersistentAndDayReset(t *testing.T) {
	p := filepath.Join(t.TempDir(), "db")
	s, _ := Open(p)
	if e := s.Spend("youtube", "2026-09-22", 2, 3); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, _ = Open(p)
	defer s.Close()
	if e := s.Spend("youtube", "2026-09-22", 2, 3); e == nil {
		t.Fatal("quota exceeded")
	}
	if e := s.Spend("youtube", "2026-09-23", 2, 3); e != nil {
		t.Fatal(e)
	}
}
func TestReusedHandleDoesNotMergeStableIDs(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "db"))
	defer s.Close()
	a, _ := model.Normalize("https://youtube.com/@example")
	a.InputURL = a.URL
	a.PlatformID = "UCaaaaaaaaaaaaaaaaaaaaaa"
	a.URL = "https://www.youtube.com/channel/" + a.PlatformID
	one, _ := s.Upsert(a, "https://example.org", time.Now())
	a.PlatformID = "UCbbbbbbbbbbbbbbbbbbbbbb"
	a.URL = "https://www.youtube.com/channel/" + a.PlatformID
	two, e := s.Upsert(a, "https://example.org", time.Now())
	if e != nil || one.ID == two.ID {
		t.Fatalf("merge: %v", e)
	}
}
