package sources

import (
	"context"
	"errors"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"net/url"
	"strings"
)

func (e *Engine) bsky(ctx context.Context, method string, params url.Values, out any) error {
	base := e.BlueskyBase
	if base == "" {
		base = "https://public.api.bsky.app"
	}
	return e.Client.JSON(ctx, "GET", base+"/xrpc/"+method+"?"+params.Encode(), nil, nil, out, nil)
}
func (e *Engine) bluesky(ctx context.Context, c Config, limit int) ([]Lead, error) {
	uri := c.URL
	if c.Kind == "bluesky_starterpack" {
		if strings.HasPrefix(uri, "https://bsky.app/starter-pack/") {
			u, _ := url.Parse(uri)
			p := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(p) != 3 {
				return nil, errors.New("invalid starter-pack URL")
			}
			var profile struct {
				DID string `json:"did"`
			}
			if err := e.bsky(ctx, "app.bsky.actor.getProfile", url.Values{"actor": {p[1]}}, &profile); err != nil {
				return nil, err
			}
			uri = "at://" + profile.DID + "/app.bsky.graph.starterpack/" + p[2]
		}
		if !strings.HasPrefix(uri, "at://") || !strings.Contains(uri, "/app.bsky.graph.starterpack/") {
			return nil, errors.New("invalid starter-pack URI")
		}
		var doc struct {
			StarterPack struct {
				Record struct {
					List string `json:"list"`
				} `json:"record"`
			} `json:"starterPack"`
		}
		if err := e.bsky(ctx, "app.bsky.graph.getStarterPack", url.Values{"starterPack": {uri}}, &doc); err != nil {
			return nil, err
		}
		uri = doc.StarterPack.Record.List
	}
	if !strings.HasPrefix(uri, "at://") || !strings.Contains(uri, "/app.bsky.graph.list/") {
		return nil, errors.New("Bluesky list requires an at://.../app.bsky.graph.list/... URI")
	}
	pages := c.MaxPages
	if pages <= 0 {
		pages = 10
	}
	cursor := ""
	cursors := map[string]bool{}
	out := []Lead{}
	seen := map[string]bool{}
	for page := 0; page < pages; page++ {
		var doc struct {
			Cursor string `json:"cursor"`
			Items  *[]struct {
				Subject struct {
					DID         string `json:"did"`
					Handle      string `json:"handle"`
					Name        string `json:"displayName"`
					Description string `json:"description"`
				} `json:"subject"`
			} `json:"items"`
		}
		params := url.Values{"list": {uri}, "limit": {"100"}}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		if err := e.bsky(ctx, "app.bsky.graph.getList", params, &doc); err != nil {
			return out, err
		}
		if doc.Items == nil {
			return out, errors.New("Bluesky schema changed: missing items")
		}
		for _, i := range *doc.Items {
			p := i.Subject
			a, err := model.Normalize("https://bsky.app/profile/" + p.DID)
			if err != nil || a.PlatformID == "" {
				return out, errors.New("Bluesky member missing DID")
			}
			if seen[a.Key()] {
				continue
			}
			if len(out) >= limit {
				return out, errors.New("Bluesky item cap reached (partial)")
			}
			seen[a.Key()] = true
			a.Name = p.Name
			a.Handle = p.Handle
			src := c.URL
			if src == "" {
				src = a.URL
			}
			out = append(out, Lead{a, src})
		}
		if doc.Cursor == "" {
			return out, nil
		}
		if cursors[doc.Cursor] {
			return out, errors.New("Bluesky repeated cursor")
		}
		cursors[doc.Cursor] = true
		cursor = doc.Cursor
	}
	return out, errors.New("Bluesky page cap reached (partial)")
}
