package sources

import (
	"context"
	"errors"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
)

func (e *Engine) youtube(ctx context.Context, resource string, params url.Values, units int, out any) error {
	if e.APIKey == "" {
		return errors.New("YouTube API key not configured")
	}
	if e.Store == nil {
		return errors.New("persistent quota ledger required")
	}
	base := e.YouTubeBase
	if base == "" {
		base = "https://www.googleapis.com/youtube/v3"
	}
	budget := e.DailyBudget
	if budget <= 0 {
		budget = 500
	}
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return err
	}
	before := func() error { return e.Store.Spend("youtube", time.Now().In(loc).Format("2006-01-02"), units, budget) }
	// Keep credentials out of query strings and error messages.
	return e.Client.JSON(ctx, "GET", base+"/"+resource+"?"+params.Encode(), map[string]string{"X-Goog-Api-Key": e.APIKey}, nil, out, before)
}
func (e *Engine) Resolve(ctx context.Context, a model.Account) (model.Account, error) {
	if a.Platform != "youtube" && a.Platform != "youtube_video" {
		return a, nil
	}
	if e.APIKey == "" {
		return a, errors.New("YouTube resolution skipped: no API key")
	}
	input := a.URL
	if e.Store != nil {
		cached, ok, err := e.Store.Cached(input, 7*24*time.Hour)
		if err != nil {
			return a, err
		}
		if ok {
			return cached, nil
		}
	}
	type snippet struct {
		Title        string `json:"title"`
		CustomURL    string `json:"customUrl"`
		Description  string `json:"description"`
		ChannelID    string `json:"channelId"`
		ChannelTitle string `json:"channelTitle"`
	}
	var response struct {
		Items []struct {
			ID      string  `json:"id"`
			Snippet snippet `json:"snippet"`
		} `json:"items"`
	}
	p := url.Values{"part": {"snippet"}}
	resource := "channels"
	if a.Platform == "youtube_video" {
		resource = "videos"
		p.Set("id", a.PlatformID)
	} else if a.PlatformID != "" {
		p.Set("id", a.PlatformID)
	} else if strings.HasPrefix(a.Handle, "@") {
		p.Set("forHandle", strings.TrimPrefix(a.Handle, "@"))
	} else if strings.Contains(a.URL, "/user/") {
		p.Set("forUsername", a.Handle)
	} else {
		return a, errors.New("legacy /c/ URL requires manual stable channel ID")
	}
	if err := e.youtube(ctx, resource, p, 1, &response); err != nil {
		return a, err
	}
	if len(response.Items) != 1 {
		return a, errors.New("YouTube account unavailable or unresolved")
	}
	item := response.Items[0]
	id := item.ID
	name := item.Snippet.Title
	description := item.Snippet.Description
	if resource == "videos" {
		id = item.Snippet.ChannelID
		name = item.Snippet.ChannelTitle
		description = ""
	}
	result, err := model.Normalize("https://www.youtube.com/channel/" + id)
	if err != nil {
		return a, errors.New("YouTube returned invalid channel ID")
	}
	result.Name = name
	result.Description = description
	result.Handle = item.Snippet.CustomURL
	result.InputURL = input
	if err = e.Store.Cache(input, result); err != nil {
		return a, err
	}
	return result, nil
}
func (e *Engine) searchYouTube(ctx context.Context, c Config, limit int) ([]Lead, error) {
	if strings.TrimSpace(c.Query) == "" {
		return nil, errors.New("YouTube search needs an explicit query")
	}
	pages := c.MaxPages
	if pages <= 0 {
		pages = 1
	}
	out := []Lead{}
	token := ""
	tokens := map[string]bool{}
	for page := 0; page < pages; page++ {
		var doc struct {
			Next  string `json:"nextPageToken"`
			Items []struct {
				ID struct {
					ChannelID string `json:"channelId"`
				} `json:"id"`
				Snippet struct {
					Title       string `json:"title"`
					Description string `json:"description"`
				} `json:"snippet"`
			} `json:"items"`
		}
		p := url.Values{"part": {"snippet"}, "type": {"channel"}, "q": {c.Query}, "maxResults": {"50"}, "relevanceLanguage": {"th"}}
		if token != "" {
			p.Set("pageToken", token)
		}
		if err := e.youtube(ctx, "search", p, 100, &doc); err != nil {
			return out, err
		}
		for _, item := range doc.Items {
			if len(out) >= limit {
				return out, errors.New("YouTube search item cap reached (partial)")
			}
			a, err := model.Normalize("https://www.youtube.com/channel/" + item.ID.ChannelID)
			if err != nil {
				return out, err
			}
			a.Name = item.Snippet.Title
			a.Description = item.Snippet.Description
			out = append(out, Lead{a, a.URL})
		}
		if doc.Next == "" {
			return out, nil
		}
		if tokens[doc.Next] {
			return out, errors.New("YouTube repeated page token")
		}
		tokens[doc.Next] = true
		token = doc.Next
	}
	return out, errors.New("YouTube search page cap reached (partial)")
}
