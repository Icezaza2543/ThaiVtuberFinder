package app

import (
	"context"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/sources"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineCycleIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "leads.jsonl")
	os.WriteFile(path, []byte("{\"url\":\"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa\",\"name\":\"Synthetic VTuberTH\"}\n"), 0600)
	c := Defaults()
	c.Database = filepath.Join(dir, "db")
	c.Sources = []sources.Config{{Name: "local", Kind: "jsonl", Path: path, Enabled: true, MaxItems: 100}}
	a, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	one, e := a.Once(context.Background())
	if e != nil || one.Stored != 1 {
		t.Fatal(e, one)
	}
	_, e = a.Once(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	rows, _ := a.Store.Candidates()
	if len(rows) != 1 {
		t.Fatal("duplicate after second cycle")
	}
}
func TestSourceFailureDoesNotStopGoodSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.jsonl")
	os.WriteFile(path, []byte("{\"url\":\"https://twitch.tv/example\"}\n"), 0600)
	c := Defaults()
	c.Database = filepath.Join(dir, "db")
	c.Sources = []sources.Config{{Name: "bad", Kind: "jsonl", Path: filepath.Join(dir, "missing"), Enabled: true}, {Name: "ok", Kind: "jsonl", Path: path, Enabled: true}}
	a, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	r, e := a.Once(context.Background())
	if e == nil || r.Stored != 1 || r.FailedSources != 1 {
		t.Fatal(e, r)
	}
}
func TestConfigurationBounds(t *testing.T) {
	c := Defaults()
	c.Workers = 0
	if c.Validate() == nil {
		t.Fatal("zero workers accepted")
	}
	c = Defaults()
	c.Interval = "0s"
	if c.Validate() == nil {
		t.Fatal("zero interval accepted")
	}
}
func TestProcessLock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "lock")
	one, e := AcquireLock(p)
	if e != nil {
		t.Fatal(e)
	}
	if two, e := AcquireLock(p); e == nil {
		two.Close()
		t.Fatal("second writer accepted")
	}
	one.Close()
	three, e := AcquireLock(p)
	if e != nil {
		t.Fatal(e)
	}
	three.Close()
}
