// Package twitch resolves Twitch logins to stable numeric user IDs via Helix.
// Results are review hints only; a login that no longer resolves is reported,
// never replaced with a similar handle.
package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var loginRe = regexp.MustCompile(`^[a-z0-9_]{1,25}$`)

type Resolver struct {
	HTTP         *http.Client
	ClientID     string
	ClientSecret string
	AuthBase     string
	APIBase      string
	token        string
}

type Result struct {
	Login       string `json:"login"`
	UserID      string `json:"user_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	URL         string `json:"url"`
	Status      string `json:"status"`
}

// Login extracts a lower-case Twitch login from a URL or bare login.
func Login(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		if h != "twitch.tv" && h != "m.twitch.tv" {
			return "", false
		}
		s = strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	}
	s = strings.ToLower(strings.TrimPrefix(s, "@"))
	return s, loginRe.MatchString(s)
}

func (r *Resolver) client() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (r *Resolver) auth(ctx context.Context) error {
	if r.token != "" {
		return nil
	}
	if r.ClientID == "" || r.ClientSecret == "" {
		return errors.New("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET required")
	}
	base := r.AuthBase
	if base == "" {
		base = "https://id.twitch.tv"
	}
	form := url.Values{"client_id": {r.ClientID}, "client_secret": {r.ClientSecret}, "grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := r.client().Do(req)
	if err != nil {
		return errors.New("twitch token request failed")
	}
	defer res.Body.Close()
	var doc struct {
		AccessToken string `json:"access_token"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&doc) != nil || doc.AccessToken == "" {
		return fmt.Errorf("twitch token request rejected: HTTP %d", res.StatusCode)
	}
	r.token = doc.AccessToken
	return nil
}

// Resolve looks up logins in batches of 100 and reports every input.
func (r *Resolver) Resolve(ctx context.Context, logins []string) ([]Result, error) {
	if err := r.auth(ctx); err != nil {
		return nil, err
	}
	base := r.APIBase
	if base == "" {
		base = "https://api.twitch.tv/helix"
	}
	found := map[string]Result{}
	for i := 0; i < len(logins); i += 100 {
		end := min(i+100, len(logins))
		q := url.Values{}
		for _, l := range logins[i:end] {
			q.Add("login", l)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", base+"/users?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Client-Id", r.ClientID)
		req.Header.Set("Authorization", "Bearer "+r.token)
		res, err := r.client().Do(req)
		if err != nil {
			return nil, errors.New("twitch users request failed")
		}
		var doc struct {
			Data []struct {
				ID          string `json:"id"`
				Login       string `json:"login"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
		}
		derr := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&doc)
		res.Body.Close()
		if res.StatusCode != 200 || derr != nil {
			return nil, fmt.Errorf("twitch users request rejected: HTTP %d", res.StatusCode)
		}
		for _, u := range doc.Data {
			found[strings.ToLower(u.Login)] = Result{Login: strings.ToLower(u.Login), UserID: u.ID, DisplayName: u.DisplayName, Status: "resolved"}
		}
	}
	out := make([]Result, 0, len(logins))
	for _, l := range logins {
		res, ok := found[l]
		if !ok {
			res = Result{Login: l, Status: "not_found"}
		}
		res.URL = "https://www.twitch.tv/" + l
		out = append(out, res)
	}
	return out, nil
}

// Channel is a Twitch broadcaster found by ThaiVTubers.
type Channel struct {
	Login, DisplayName, UserID, How string
}

var virtualSignal = regexp.MustCompile(`(?i)vtuber|vtube|v-tuber|pngtuber|vsinger|วีทูป|วีทูบ|live2d`)

func (r *Resolver) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Client-Id", r.ClientID)
	req.Header.Set("Authorization", "Bearer "+r.token)
	res, err := r.client().Do(req)
	if err != nil {
		return errors.New("twitch request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("twitch request rejected: HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out)
}

// ThaiVTubers returns broadcasters with language "th" whose tags/title show a
// virtual-creator signal: current Thai live streams plus channel search results.
// Each call is a snapshot; running it every worker cycle accumulates coverage.
func (r *Resolver) ThaiVTubers(ctx context.Context, queries []string, maxPages int) ([]Channel, error) {
	if err := r.auth(ctx); err != nil {
		return nil, err
	}
	base := r.APIBase
	if base == "" {
		base = "https://api.twitch.tv/helix"
	}
	seen := map[string]Channel{}
	cursor := ""
	for page := 0; page < maxPages; page++ {
		var raw struct {
			Data []struct {
				UserLogin string   `json:"user_login"`
				UserName  string   `json:"user_name"`
				UserID    string   `json:"user_id"`
				Title     string   `json:"title"`
				Tags      []string `json:"tags"`
			} `json:"data"`
			Pagination struct {
				Cursor string `json:"cursor"`
			} `json:"pagination"`
		}
		q := url.Values{"language": {"th"}, "first": {"100"}}
		if cursor != "" {
			q.Set("after", cursor)
		}
		if err := r.getJSON(ctx, base+"/streams?"+q.Encode(), &raw); err != nil {
			return nil, err
		}
		for _, s := range raw.Data {
			if virtualSignal.MatchString(s.Title + " " + strings.Join(s.Tags, " ")) {
				seen[strings.ToLower(s.UserLogin)] = Channel{strings.ToLower(s.UserLogin), s.UserName, s.UserID, "live stream (th)"}
			}
		}
		if cursor = raw.Pagination.Cursor; cursor == "" {
			break
		}
	}
	for _, query := range queries {
		var raw struct {
			Data []struct {
				Login    string   `json:"broadcaster_login"`
				Name     string   `json:"display_name"`
				ID       string   `json:"id"`
				Language string   `json:"broadcaster_language"`
				Title    string   `json:"title"`
				Tags     []string `json:"tags"`
			} `json:"data"`
		}
		q := url.Values{"query": {query}, "first": {"100"}}
		if err := r.getJSON(ctx, base+"/search/channels?"+q.Encode(), &raw); err != nil {
			return nil, err
		}
		for _, c := range raw.Data {
			text := c.Name + " " + c.Title + " " + strings.Join(c.Tags, " ")
			if c.Language == "th" && virtualSignal.MatchString(text) {
				login := strings.ToLower(c.Login)
				if _, ok := seen[login]; !ok {
					seen[login] = Channel{login, c.Name, c.ID, "search " + query}
				}
			}
		}
	}
	out := make([]Channel, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	return out, nil
}
