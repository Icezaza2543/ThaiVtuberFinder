// Package sources reads only explicitly configured public creator directories.
package sources

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/enrich"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/store"
	"html"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Path     string `json:"path"`
	Query    string `json:"query"`
	Enabled  bool   `json:"enabled"`
	MaxPages int    `json:"max_pages"`
	MaxItems int    `json:"max_items"`
}
type Lead struct {
	Account   model.Account
	SourceURL string
}
type Engine struct {
	Client      *netx.Client
	Store       *store.Store
	APIKey      string
	DailyBudget int
	YouTubeBase string
	BlueskyBase string
}

func (e *Engine) Discover(ctx context.Context, c Config) ([]Lead, error) {
	limit := c.MaxItems
	if limit <= 0 {
		limit = 10000
	}
	switch c.Kind {
	case "kerlos":
		raw := c.URL
		if raw == "" {
			raw = "https://storage.googleapis.com/thaivtuberranking.appspot.com/v2/channel_data/simple_list.json"
		}
		b, err := e.Client.Bytes(ctx, "GET", raw, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		return ParseDirectory(b, raw, limit)
	case "html":
		b, err := e.Client.Bytes(ctx, "GET", c.URL, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		return ParseHTML(string(b), c.URL, limit)
	case "jsonl", "csv", "json":
		f, err := os.Open(c.Path)
		if err != nil {
			return nil, fmt.Errorf("source file unavailable: %s", c.Name)
		}
		defer f.Close()
		info, statErr := f.Stat()
		if statErr != nil {
			return nil, statErr
		}
		if info.Size() > 32<<20 {
			return nil, errors.New("source file exceeds 32 MB")
		}
		r := io.LimitReader(f, 32<<20)
		if c.Kind == "json" {
			return ParseJSON(r, limit)
		}
		if c.Kind == "jsonl" {
			return ParseJSONL(r, limit)
		}
		return parseCSV(r, limit)
	case "bluesky_list", "bluesky_starterpack":
		return e.bluesky(ctx, c, limit)
	case "raw_links":
		b, err := e.Client.Bytes(ctx, "GET", c.URL, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		return ParseRawLinks(string(b), c.URL, limit)
	case "vtuberthaiinfo":
		b, err := e.Client.Bytes(ctx, "GET", c.URL, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		return ParseVtuberThaiInfo(string(b), c.URL, limit)
	case "tipjai":
		return e.tipjai(ctx, c, limit)
	case "youtube_search":
		return e.searchYouTube(ctx, c, limit)
	default:
		return nil, fmt.Errorf("unsupported source kind: %s", c.Kind)
	}
}

func ParseRawLinks(body, source string, limit int) ([]Lead, error) {
	links := enrich.ExtractLinks(body)
	out := make([]Lead, 0, len(links))
	seen := map[string]bool{}
	for _, raw := range links {
		a, err := model.Normalize(raw)
		if err != nil || a.Platform == "" || seen[a.Key()] {
			continue
		}
		seen[a.Key()] = true
		a.ClassificationHint = "ORGANIZATION_ROSTER"
		a.ThaiRelevanceHint = "thai_signal"
		out = append(out, Lead{Account: a, SourceURL: source})
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no supported account links found in source")
	}
	return out, nil
}

var nextPush = regexp.MustCompile(`self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`)

// ParseVtuberThaiInfo reads the talent list embedded in the Next.js payload of
// the VtuberThaiInfo archive. The listing is a discovery hint only.
func ParseVtuberThaiInfo(body, source string, limit int) ([]Lead, error) {
	var payload strings.Builder
	for _, m := range nextPush.FindAllStringSubmatch(body, -1) {
		var chunk string
		if json.Unmarshal([]byte(m[1]), &chunk) == nil {
			payload.WriteString(chunk)
		}
	}
	text := payload.String()
	i := strings.Index(text, `{"talents":`)
	if i < 0 {
		return nil, errors.New("vtuberthaiinfo schema changed: talents payload not found")
	}
	type channel struct {
		Username    string `json:"username"`
		ChannelName string `json:"channelName"`
		ChannelID   string `json:"channelId"`
	}
	var doc struct {
		Talents *[]struct {
			Name        string   `json:"name"`
			YouTubeMain *channel `json:"youtubeMain"`
			TwitchMain  *channel `json:"twitchMain"`
		} `json:"talents"`
	}
	if err := json.NewDecoder(strings.NewReader(text[i:])).Decode(&doc); err != nil || doc.Talents == nil {
		return nil, errors.New("vtuberthaiinfo schema changed: expected talents[]")
	}
	out := []Lead{}
	seen := map[string]bool{}
	add := func(raw, name string) {
		a, err := model.Normalize(raw)
		if err != nil || a.Platform == "" || seen[a.Key()] {
			return
		}
		seen[a.Key()] = true
		a.Name = name
		a.ClassificationHint = "DIRECTORY_LISTED"
		out = append(out, Lead{a, source})
	}
	for _, t := range *doc.Talents {
		if y := t.YouTubeMain; y != nil && strings.HasPrefix(y.ChannelID, "UC") {
			add("https://www.youtube.com/channel/"+y.ChannelID, firstNonEmpty(y.ChannelName, t.Name))
		}
		if tw := t.TwitchMain; tw != nil && tw.Username != "" {
			add("https://www.twitch.tv/"+tw.Username, firstNonEmpty(tw.ChannelName, t.Name))
		}
		if len(out) >= limit {
			return out[:limit], errors.New("vtuberthaiinfo item cap reached (partial)")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("vtuberthaiinfo unexpectedly empty")
	}
	return out, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func ParseDirectory(b []byte, source string, limit int) ([]Lead, error) {
	var doc struct {
		Result *[]struct {
			ID    string `json:"channel_id"`
			Title string `json:"title"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Result == nil {
		return nil, errors.New("directory schema changed: expected result[]")
	}
	out := []Lead{}
	for _, r := range *doc.Result {
		a, err := model.Normalize("https://www.youtube.com/channel/" + r.ID)
		if err != nil {
			return out, errors.New("invalid channel ID in directory")
		}
		a.Name = r.Title
		a.ClassificationHint = "DIRECTORY_LISTED"
		out = append(out, Lead{a, source})
		if len(out) >= limit && len(out) < len(*doc.Result) {
			return out, errors.New("directory item cap reached (partial)")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("directory unexpectedly empty")
	}
	return out, nil
}

var anchors = regexp.MustCompile(`(?is)<a\b[^>]*\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a\s*>`)
var tags = regexp.MustCompile(`<[^>]+>`)

func ParseHTML(body, source string, limit int) ([]Lead, error) {
	base, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	out := []Lead{}
	seen := map[string]bool{}
	for _, m := range anchors.FindAllStringSubmatch(body, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		a, err := model.Normalize(base.ResolveReference(u).String())
		if err != nil || seen[a.Key()] {
			continue
		}
		seen[a.Key()] = true
		a.Name = strings.TrimSpace(html.UnescapeString(tags.ReplaceAllString(m[2], " ")))
		out = append(out, Lead{a, source})
		if len(out) >= limit {
			return out, errors.New("HTML item cap reached (partial)")
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no supported account links; page may require JavaScript or manual adapter")
	}
	return out, nil
}

type importRow struct {
	URL          string `json:"url"`
	CanonicalURL string `json:"canonical_url"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	SourceURL    string `json:"source_url"`
	Description  string `json:"description"`
}

func leadFromRow(r importRow) (Lead, error) {
	raw := r.CanonicalURL
	if raw == "" {
		raw = r.URL
	}
	a, e := model.Normalize(raw)
	if e != nil {
		return Lead{}, e
	}
	a.Name = r.DisplayName
	if a.Name == "" {
		a.Name = r.Name
	}
	a.Description = r.Description
	source := r.SourceURL
	if source == "" {
		source = a.URL
	}
	u, e := url.Parse(source)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return Lead{}, errors.New("source_url must be public HTTPS")
	}
	return Lead{a, source}, nil
}
func ParseJSONL(r io.Reader, limit int) ([]Lead, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 4096), 1<<20)
	out := []Lead{}
	line := 0
	for sc.Scan() {
		line++
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		if len(out) >= limit {
			return out, errors.New("JSONL item cap reached (partial)")
		}
		var row importRow
		if e := json.Unmarshal(sc.Bytes(), &row); e != nil {
			return out, fmt.Errorf("invalid JSONL at line %d", line)
		}
		lead, e := leadFromRow(row)
		if e != nil {
			return out, fmt.Errorf("invalid account at line %d", line)
		}
		out = append(out, lead)
	}
	return out, sc.Err()
}
func parseCSV(r io.Reader, limit int) ([]Lead, error) {
	rd := csv.NewReader(r)
	head, e := rd.Read()
	if e != nil {
		return nil, e
	}
	out := []Lead{}
	line := 1
	for {
		row, e := rd.Read()
		if e == io.EOF {
			break
		}
		line++
		if e != nil {
			return out, fmt.Errorf("invalid CSV line %d", line)
		}
		if len(out) >= limit {
			return out, errors.New("CSV cap reached (partial)")
		}
		m := map[string]string{}
		for i, k := range head {
			m[k] = row[i]
		}
		lead, e := leadFromRow(importRow{URL: m["url"], CanonicalURL: m["canonical_url"], Name: m["name"], DisplayName: m["display_name"], SourceURL: m["source_url"], Description: m["description"]})
		if e != nil {
			return out, fmt.Errorf("invalid CSV account at line %d", line)
		}
		out = append(out, lead)
	}
	return out, nil
}

// ParseJSON accepts an array or a {results: [...]} screening export.
// Invalid URLs are reported after processing the remaining valid records.
func ParseJSON(r io.Reader, limit int) ([]Lead, error) {
	b, e := io.ReadAll(r)
	if e != nil {
		return nil, e
	}
	var records []importRow
	if e = json.Unmarshal(b, &records); e != nil {
		var doc struct {
			Results []importRow `json:"results"`
		}
		if e = json.Unmarshal(b, &doc); e != nil || doc.Results == nil {
			return nil, errors.New("expected JSON array or results[]")
		}
		records = doc.Results
	}
	out := []Lead{}
	invalid := 0
	for _, r := range records {
		if len(out) >= limit {
			return out, errors.New("JSON item cap reached (partial)")
		}
		lead, e := leadFromRow(r)
		if e != nil {
			invalid++
			continue
		}
		out = append(out, lead)
	}
	if invalid > 0 {
		return out, fmt.Errorf("%d imported records have unsupported/malformed account URLs", invalid)
	}
	return out, nil
}
