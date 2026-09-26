package sources

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
)

// Tipjai (tipjai.com) is a Thai donation platform with a public creator
// directory (/discover/<category>). robots.txt allows creator pages. Only the
// slug, display name and owner-entered social links are read; PromptPay or
// other payment details on the page are never extracted.

var (
	tipjaiSlugHref = regexp.MustCompile(`href="/([A-Za-z0-9_-]{2,40})"`)
	tipjaiTitle    = regexp.MustCompile(`<title>(?:โดเนท\s+)?(.*?)\s*\(@([A-Za-z0-9_-]+)\)`)
	socialHref     = regexp.MustCompile(`https?://(?:www\.|m\.)?(?:youtube\.com|youtu\.be|twitch\.tv|x\.com|twitter\.com|tiktok\.com|facebook\.com|instagram\.com|bsky\.app)/[^"'\\\s<>]+`)
)

var tipjaiReserved = map[string]bool{"discover": true, "blog": true, "for": true, "creator-fund": true,
	"dashboard": true, "onboarding": true, "api": true, "overlay": true, "auth": true, "verify": true,
	"link": true, "login": true, "signup": true, "terms": true, "privacy": true, "about": true,
	"pricing": true, "help": true, "docs": true, "guide": true, "changelog": true, "brand": true,
	"feedback": true, "investors": true, "how-it-works": true, "discord": true}

// ParseTipjaiDirectory returns creator slugs linked from a /discover page.
func ParseTipjaiDirectory(body string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range tipjaiSlugHref.FindAllStringSubmatch(body, -1) {
		s := strings.ToLower(m[1])
		if tipjaiReserved[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// SiteSocialLinks returns social links present on a non-creator page (site
// header/footer), which must never be attributed to a creator.
func SiteSocialLinks(body string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range socialHref.FindAllString(body, -1) {
		out[html.UnescapeString(raw)] = true
	}
	return out
}

// ParseTipjaiCreator returns the Tipjai account plus the owner's social links.
// Pages whose title is not "<name> (@<slug>)" are not creator pages and yield
// nothing. Links in siteLinks (site chrome) and Tipjai's own accounts are excluded.
func ParseTipjaiCreator(body, slug, source string, siteLinks map[string]bool) []Lead {
	m := tipjaiTitle.FindStringSubmatch(body)
	if m == nil || !strings.EqualFold(m[2], slug) {
		return nil
	}
	name := html.UnescapeString(strings.TrimSpace(m[1]))
	leads := []Lead{}
	if a, err := model.Normalize("https://tipjai.com/" + slug); err == nil {
		a.Name = name
		a.ClassificationHint = "DIRECTORY_LISTED"
		leads = append(leads, Lead{a, source})
	}
	seen := map[string]bool{}
	for _, raw := range socialHref.FindAllString(body, -1) {
		raw = html.UnescapeString(raw)
		if siteLinks[raw] || strings.Contains(strings.ToLower(raw), "tipjai") {
			continue
		}
		a, err := model.Normalize(raw)
		if err != nil || a.Platform == "" || seen[a.Key()] {
			continue
		}
		seen[a.Key()] = true
		a.Name = name
		a.ClassificationHint = "DIRECTORY_LISTED"
		leads = append(leads, Lead{a, source})
	}
	return leads
}

func (e *Engine) tipjai(ctx context.Context, c Config, limit int) ([]Lead, error) {
	dir := c.URL
	if dir == "" {
		dir = "https://tipjai.com/discover/vtuber"
	}
	b, err := e.Client.Bytes(ctx, "GET", dir, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	slugs := ParseTipjaiDirectory(string(b))
	site := SiteSocialLinks(string(b))
	if len(slugs) == 0 {
		return nil, errors.New("tipjai schema changed: no creator links on directory page")
	}
	if c.MaxPages > 0 && len(slugs) > c.MaxPages {
		slugs = slugs[:c.MaxPages]
	}
	out := []Lead{}
	var failed int
	for i, s := range slugs {
		if i > 0 {
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(750 * time.Millisecond): // polite pacing
			}
		}
		page := "https://tipjai.com/" + s
		body, err := e.Client.Bytes(ctx, "GET", page, nil, nil, nil)
		if err != nil {
			failed++
			continue
		}
		out = append(out, ParseTipjaiCreator(string(body), s, page, site)...)
		if len(out) >= limit {
			return out[:limit], errors.New("tipjai item cap reached (partial)")
		}
	}
	if failed > 0 {
		return out, fmt.Errorf("tipjai: %d creator pages failed", failed)
	}
	return out, nil
}
