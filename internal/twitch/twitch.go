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
