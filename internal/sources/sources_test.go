package sources

import (
	"context"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRosterLinks(t *testing.T) {
	leads, e := ParseHTML(`<a href="https://youtube.com/@Example">Example VTuberTH</a><a href='https://youtube.com/@Example/videos'>same</a><a href="https://youtube.com.evil.test/@bad">bad</a>`, "https://event.example", 20)
	if e != nil || len(leads) != 1 || leads[0].Account.Platform != "youtube" {
		t.Fatalf("%v %+v", e, leads)
	}
}
func TestParseRawLinks(t *testing.T) {
	raw := `{"members": ["https://youtube.com/@Example", "https://twitch.tv/ExampleTwitch"]}`
	leads, err := ParseRawLinks(raw, "https://api.example.com/roster.json", 10)
	if err != nil || len(leads) != 2 {
		t.Fatalf("unexpected leads: %v, len=%d", err, len(leads))
	}
	if leads[0].Account.Platform != "youtube" || leads[1].Account.Platform != "twitch" {
		t.Fatalf("unexpected accounts: %+v", leads)
	}
}
func TestDirectoryShape(t *testing.T) {
	leads, e := ParseDirectory([]byte(`{"result":[{"channel_id":"UCaaaaaaaaaaaaaaaaaaaaaa","title":"Example VTuberTH"}]}`), "https://example.org/feed", 10)
	if e != nil || len(leads) != 1 {
		t.Fatal(e, leads)
	}
	if _, e = ParseDirectory([]byte(`{"changed_shape":[]}`), "https://example.org", 10); e == nil {
		t.Fatal("shape drift hidden")
	}
}
func TestJSONLImport(t *testing.T) {
	leads, e := ParseJSONL(strings.NewReader("{\"url\":\"https://twitch.tv/Example\",\"name\":\"Example\"}\n"), 10)
	if e != nil || len(leads) != 1 {
		t.Fatal(e)
	}
	if _, e = ParseJSONL(strings.NewReader("{broken}\n"), 10); e == nil {
		t.Fatal("malformed imported")
	}
}
func TestBlueskyPagination(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("cursor") == "" {
			w.Write([]byte(`{"items":[{"subject":{"did":"did:plc:one","handle":"one.bsky.social","displayName":"One VTuberTH"}}],"cursor":"next"}`))
		} else {
			w.Write([]byte(`{"items":[{"subject":{"did":"did:plc:two","handle":"two.bsky.social"}}]}`))
		}
	}))
	defer s.Close()
	e := Engine{Client: &netx.Client{HTTP: s.Client()}, BlueskyBase: s.URL}
	items, err := e.Discover(context.Background(), Config{Name: "test", Kind: "bluesky_list", URL: "at://did:plc:owner/app.bsky.graph.list/key", MaxPages: 3, MaxItems: 20})
	if err != nil || len(items) != 2 || calls != 2 {
		t.Fatal(err, len(items), calls)
	}
}
func TestYouTubeHandleResolutionAndQuota(t *testing.T) {
	calls := 0
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("forHandle") != "example" {
			t.Error("missing handle")
		}
		w.Write([]byte(`{"items":[{"id":"UCaaaaaaaaaaaaaaaaaaaaaa","snippet":{"title":"Example VTuberTH","customUrl":"@example","description":"Thai VTuber"}}]}`))
	}))
	defer sv.Close()
	db, _ := store.Open(filepath.Join(t.TempDir(), "db"))
	defer db.Close()
	eng := Engine{Client: &netx.Client{HTTP: sv.Client()}, Store: db, YouTubeBase: sv.URL, APIKey: "fake-test-key", DailyBudget: 1}
	a, _ := model.Normalize("https://youtube.com/@example")
	resolved, err := eng.Resolve(context.Background(), a)
	if err != nil || resolved.PlatformID == "" {
		t.Fatal(err)
	}
	_, err = eng.Resolve(context.Background(), a)
	if err != nil || calls != 1 {
		t.Fatal("cache missed", err, calls)
	}
}
func TestBlueskyPageCapIsReported(t *testing.T) {
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"items":[],"cursor":"more"}`)) }))
	defer sv.Close()
	eng := Engine{Client: &netx.Client{HTTP: sv.Client()}, BlueskyBase: sv.URL}
	_, e := eng.Discover(context.Background(), Config{Kind: "bluesky_list", URL: "at://did:plc:owner/app.bsky.graph.list/key", MaxPages: 1, MaxItems: 20})
	if e == nil {
		t.Fatal("partial treated as complete")
	}
}
func TestResolveCachePreservesClassificationSignal(t *testing.T) {
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[{"id":"UCaaaaaaaaaaaaaaaaaaaaaa","snippet":{"title":"Example","customUrl":"@example","description":"Thai VTuberTH"}}]}`))
	}))
	defer sv.Close()
	db, _ := store.Open(filepath.Join(t.TempDir(), "db"))
	defer db.Close()
	eng := Engine{Client: &netx.Client{HTTP: sv.Client()}, Store: db, YouTubeBase: sv.URL, APIKey: "test", DailyBudget: 10}
	a, _ := model.Normalize("https://youtube.com/@example")
	first, e := eng.Resolve(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	one, e := db.Upsert(first, "https://example.org", time.Now())
	if e != nil {
		t.Fatal(e)
	}
	cached, e := eng.Resolve(context.Background(), a)
	if e != nil {
		t.Fatal(e)
	}
	two, e := db.Upsert(cached, "https://example.org", time.Now())
	if e != nil || one.Classification != two.Classification {
		t.Fatal("cache lost signal", e, one.Classification, two.Classification)
	}
}
