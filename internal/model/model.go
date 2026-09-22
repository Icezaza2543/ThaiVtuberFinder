// Package model separates platform account identity from persona identity.
package model

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Account struct {
	Platform           string `json:"platform"`
	PlatformID         string `json:"platform_id"`
	Handle             string `json:"handle"`
	Name               string `json:"display_name"`
	URL                string `json:"canonical_url"`
	InputURL           string `json:"input_url,omitempty"`
	Description        string `json:"-"`
	ClassificationHint string `json:"classification_hint,omitempty"`
	ThaiRelevanceHint  string `json:"thai_relevance_hint,omitempty"`
}
type Candidate struct {
	Account
	ID             string `json:"candidate_id"`
	Classification string `json:"classification"`
	ThaiRelevance  string `json:"thai_relevance"`
	SourceURL      string `json:"source_url"`
	FirstSeen      string `json:"first_seen"`
	LastChecked    string `json:"last_checked_at"`
	MatchState     string `json:"match_state"`
}

type CandidateSource struct {
	CandidateID  string `json:"candidate_id"`
	SourceName   string `json:"source_name"`
	SourceURL    string `json:"source_url"`
	DiscoveredAt string `json:"discovered_at"`
}

type RelationProposal struct {
	ProposalID      string `json:"proposal_id"`
	FromCandidateID string `json:"from_candidate_id"`
	FromPlatform    string `json:"from_platform"`
	FromPlatformID  string `json:"from_platform_id"`
	ToPlatform      string `json:"to_platform"`
	ToPlatformID    string `json:"to_platform_id"`
	ToURL           string `json:"to_url"`
	EvidenceURL     string `json:"evidence_url"`
	EvidenceType    string `json:"evidence_type"`
	Confidence      string `json:"confidence"`
	CreatedAt       string `json:"created_at"`
	ReviewStatus    string `json:"review_status"`
}

var YouTubeID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var handle = regexp.MustCompile(`^[\p{L}\p{N}_.-]+$`)

func (a Account) Key() string {
	pid := strings.TrimSpace(a.PlatformID)
	if pid != "" {
		return a.Platform + ":id:" + pid
	}
	if a.Platform == "twitch" {
		h := strings.TrimSpace(a.Handle)
		if h == "" && a.URL != "" {
			if norm, err := Normalize(a.URL); err == nil && norm.Platform == "twitch" {
				return "twitch:url:" + norm.URL
			}
		}
		if h != "" {
			h = strings.ToLower(strings.TrimPrefix(h, "@"))
			return "twitch:url:https://www.twitch.tv/" + h
		}
	}
	if a.URL != "" {
		if norm, err := Normalize(a.URL); err == nil && norm.URL != "" {
			if norm.PlatformID != "" {
				return norm.Platform + ":id:" + norm.PlatformID
			}
			return norm.Platform + ":url:" + norm.URL
		}
	}
	u := strings.TrimRight(strings.TrimSpace(a.URL), "/")
	return a.Platform + ":url:" + u
}
func Normalize(raw string) (Account, error) {
	a := Account{}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Port() != "" {
		return a, fmt.Errorf("invalid public account URL")
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimPrefix(host, "m.")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	p := parts[0]
	reject := func() (Account, error) { return Account{}, fmt.Errorf("unsupported account URL") }
	profile := func(platform, h, base string) (Account, error) {
		h = strings.ToLower(h)
		if !handle.MatchString(strings.TrimPrefix(h, "@")) {
			return reject()
		}
		a.Platform = platform
		a.Handle = h
		a.URL = base + url.PathEscape(h)
		a.InputURL = a.URL
		return a, nil
	}
	switch host {
	case "youtu.be":
		if !videoID.MatchString(p) {
			return reject()
		}
		a.Platform = "youtube_video"
		a.PlatformID = p
		a.URL = "https://www.youtube.com/watch?v=" + p
	case "youtube.com":
		if p == "channel" && len(parts) >= 2 {
			if !YouTubeID.MatchString(parts[1]) {
				return reject()
			}
			a.Platform = "youtube"
			a.PlatformID = parts[1]
			a.URL = "https://www.youtube.com/channel/" + parts[1]
		} else if strings.HasPrefix(p, "@") {
			return profile("youtube", p, "https://www.youtube.com/")
		} else if (p == "c" || p == "user") && len(parts) >= 2 {
			a.Platform = "youtube"
			a.Handle = parts[1]
			a.URL = "https://www.youtube.com/" + p + "/" + url.PathEscape(parts[1])
		} else {
			v := u.Query().Get("v")
			if (p == "shorts" || p == "live") && len(parts) >= 2 {
				v = parts[1]
			}
			if !videoID.MatchString(v) {
				return reject()
			}
			a.Platform = "youtube_video"
			a.PlatformID = v
			a.URL = "https://www.youtube.com/watch?v=" + v
		}
	case "x.com", "twitter.com":
		if strings.Contains("|home|explore|search|intent|i|settings|", "|"+strings.ToLower(p)+"|") {
			return reject()
		}
		return profile("x", p, "https://x.com/")
	case "twitch.tv":
		if strings.Contains("|directory|videos|downloads|settings|jobs|p|", "|"+strings.ToLower(p)+"|") {
			return reject()
		}
		return profile("twitch", p, "https://www.twitch.tv/")
	case "tiktok.com":
		if !strings.HasPrefix(p, "@") {
			return reject()
		}
		return profile("tiktok", p, "https://www.tiktok.com/")
	case "instagram.com":
		if p == "p" || p == "reel" || p == "explore" || p == "accounts" {
			return reject()
		}
		return profile("instagram", p, "https://www.instagram.com/")
	case "facebook.com":
		if p == "profile.php" {
			id := u.Query().Get("id")
			if ok, _ := regexp.MatchString(`^[0-9]+$`, id); !ok {
				return reject()
			}
			a.Platform = "facebook"
			a.PlatformID = id
			a.URL = "https://www.facebook.com/profile.php?id=" + id
		} else {
			if p == "watch" || p == "groups" || p == "login" || p == "share" {
				return reject()
			}
			return profile("facebook", p, "https://www.facebook.com/")
		}
	case "bsky.app":
		if p != "profile" || len(parts) < 2 {
			return reject()
		}
		a.Platform = "bluesky"
		value := parts[1]
		if strings.HasPrefix(value, "did:plc:") || strings.HasPrefix(value, "did:web:") {
			a.PlatformID = value
		} else {
			if !handle.MatchString(value) {
				return reject()
			}
			a.Handle = strings.ToLower(value)
			value = a.Handle
		}
		a.URL = "https://bsky.app/profile/" + value
	default:
		return reject()
	}
	a.InputURL = a.URL
	return a, nil
}

// Classify suggests a review priority. It never verifies an account or a persona.
func Classify(description, name string) string {
	s := strings.ToLower(description + " " + name)
	for _, v := range []string{"vtuber agency", "virtual agency", "ค่ายวีทูป", "official agency"} {
		if strings.Contains(s, v) {
			return "ORGANIZATION_SIGNAL"
		}
	}
	for _, v := range []string{"fan account", "fan channel", "clipper", "ช่องแปล", "ช่องตัดคลิป"} {
		if strings.Contains(s, v) {
			return "NON_PERSONA_SIGNAL"
		}
	}
	for _, v := range []string{"vtuber", "pngtuber", "vsinger", "วีทูป", "วีทูบ", "virtual youtuber"} {
		if strings.Contains(s, v) {
			return "VTUBER_SIGNAL"
		}
	}
	return "UNRESOLVED"
}
func ThaiSignal(s string) string {
	s = strings.ToLower(s)
	for _, v := range []string{"vtuberth", "pngtuberth", "thailand", "วีทูป", "วีทูบ", "ไทย"} {
		if strings.Contains(s, v) {
			return "thai_signal"
		}
	}
	return "uncertain"
}
