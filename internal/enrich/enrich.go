package enrich

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
)

var (
	urlRegex      = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>()"']+|\b(?:youtube\.com|youtu\.be|twitch\.tv|twitter\.com|x\.com|tiktok\.com|instagram\.com|facebook\.com|[a-z0-9_-]+\.carrd\.co|linktr\.ee|lit\.link|bio\.site)/[^\s<>()"']*`)
	leadingPunct  = regexp.MustCompile(`^[\s"'<(\[{]+`)
	trailingPunct = regexp.MustCompile(`[\s"'.,:;!?)\]}>]+$`)
)

type CandidateStore interface {
	Upsert(a model.Account, source string, now time.Time) (model.Candidate, error)
	AddRelationProposal(p model.RelationProposal) error
}

type EnrichReport struct {
	ProfilesScanned         int                      `json:"profiles_scanned"`
	ProfilesWithLinks       int                      `json:"profiles_with_links"`
	YouTubeLinksFound       int                      `json:"youtube_links_found"`
	TwitchLinksFound        int                      `json:"twitch_links_found"`
	XLinksFound             int                      `json:"x_links_found"`
	OtherLinksFound         int                      `json:"other_links_found"`
	StableIDsResolved       int                      `json:"stable_ids_resolved"`
	ExistingAccountsMatched int                      `json:"existing_accounts_matched"`
	NewAccountCandidates    int                      `json:"new_account_candidates"`
	RelationProposalsCount  int                      `json:"relation_proposals"`
	Proposals               []model.RelationProposal `json:"proposals,omitempty"`
}

type Enricher struct {
	Client          *netx.Client
	Store           CandidateStore
	BlueskyBase     string
	KnownCanonical  map[string]string
	KnownInbox      map[string]string
	YouTubeResolver func(ctx context.Context, a model.Account) (model.Account, error)
}

func ExtractLinks(text string) []string {
	raw := urlRegex.FindAllString(text, -1)
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, m := range raw {
		clean := leadingPunct.ReplaceAllString(m, "")
		clean = trailingPunct.ReplaceAllString(clean, "")
		if clean == "" {
			continue
		}
		if !strings.HasPrefix(clean, "http://") && !strings.HasPrefix(clean, "https://") {
			clean = "https://" + clean
		}
		u, err := url.Parse(clean)
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "bsky.app" || strings.HasSuffix(host, ".bsky.social") {
			continue
		}
		if u.Path == "/" && u.RawQuery == "" && u.Fragment == "" {
			clean = strings.TrimRight(clean, "/")
		}
		canonicalURL := strings.TrimRight(clean, "/")
		if seen[canonicalURL] {
			continue
		}
		seen[canonicalURL] = true
		out = append(out, clean)
	}
	return out
}

func NormalizeLink(raw string) (*model.Account, string, error) {
	clean := strings.TrimSpace(raw)
	u, err := url.Parse(clean)
	if err != nil || u.Host == "" {
		return nil, "", fmt.Errorf("invalid url: %s", raw)
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")

	// Check if bio link aggregator
	if strings.HasSuffix(host, ".carrd.co") || host == "linktr.ee" || host == "lit.link" || host == "bio.site" {
		acc := &model.Account{
			Platform: "website",
			URL:      strings.TrimRight(clean, "/"),
			InputURL: raw,
		}
		return acc, "medium", nil
	}

	norm, err := model.Normalize(clean)
	if err == nil && norm.Platform != "" {
		return &norm, "high", nil
	}

	// Fallback generic website if valid http/https and not a search or social homepage
	if (u.Scheme == "http" || u.Scheme == "https") && !strings.Contains(host, "google.") && !strings.Contains(host, "bing.") {
		cleanURL := fmt.Sprintf("%s://%s%s", u.Scheme, host, u.Path)
		if u.RawQuery != "" {
			cleanURL += "?" + u.RawQuery
		}
		acc := &model.Account{
			Platform: "website",
			URL:      strings.TrimRight(cleanURL, "/"),
			InputURL: raw,
		}
		return acc, "medium", nil
	}

	return nil, "", fmt.Errorf("unsupported link: %s", raw)
}

func (e *Enricher) FetchBlueskyProfiles(ctx context.Context, dids []string) (map[string]struct {
	DID         string
	Handle      string
	DisplayName string
	Description string
}, error) {
	out := map[string]struct {
		DID         string
		Handle      string
		DisplayName string
		Description string
	}{}
	if len(dids) == 0 {
		return out, nil
	}
	base := e.BlueskyBase
	if base == "" {
		base = "https://public.api.bsky.app"
	}
	batchSize := 25
	for i := 0; i < len(dids); i += batchSize {
		end := i + batchSize
		if end > len(dids) {
			end = len(dids)
		}
		batch := dids[i:end]
		params := url.Values{}
		for _, did := range batch {
			params.Add("actors", did)
		}
		var doc struct {
			Profiles []struct {
				DID         string `json:"did"`
				Handle      string `json:"handle"`
				DisplayName string `json:"displayName"`
				Description string `json:"description"`
			} `json:"profiles"`
		}
		err := e.Client.JSON(ctx, "GET", base+"/xrpc/app.bsky.actor.getProfiles?"+params.Encode(), nil, nil, &doc, nil)
		if err != nil {
			return out, err
		}
		for _, p := range doc.Profiles {
			out[p.DID] = struct {
				DID         string
				Handle      string
				DisplayName string
				Description string
			}{
				DID:         p.DID,
				Handle:      p.Handle,
				DisplayName: p.DisplayName,
				Description: p.Description,
			}
		}
	}
	return out, nil
}

func (e *Enricher) RunBatch(ctx context.Context, blueskyCandidates []model.Candidate, limit, offset int) (*EnrichReport, error) {
	rep := &EnrichReport{
		Proposals: make([]model.RelationProposal, 0),
	}
	if offset >= len(blueskyCandidates) {
		return rep, nil
	}
	end := offset + limit
	if end > len(blueskyCandidates) {
		end = len(blueskyCandidates)
	}
	targetCandidates := blueskyCandidates[offset:end]
	dids := make([]string, 0, len(targetCandidates))
	candByDID := map[string]model.Candidate{}
	for _, c := range targetCandidates {
		if c.PlatformID != "" {
			dids = append(dids, c.PlatformID)
			candByDID[c.PlatformID] = c
		}
	}

	profiles, err := e.FetchBlueskyProfiles(ctx, dids)
	if err != nil {
		return rep, fmt.Errorf("fetch bluesky profiles error: %w", err)
	}

	now := time.Now().UTC()
	for _, c := range targetCandidates {
		rep.ProfilesScanned++
		prof, found := profiles[c.PlatformID]
		if !found || strings.TrimSpace(prof.Description) == "" {
			continue
		}
		links := ExtractLinks(prof.Description)
		if len(links) == 0 {
			continue
		}
		rep.ProfilesWithLinks++

		for _, rawLink := range links {
			targetAcc, conf, err := NormalizeLink(rawLink)
			if err != nil || targetAcc == nil {
				continue
			}

			// Low confidence is never auto-proposed per rule
			if conf == "low" {
				continue
			}

			switch targetAcc.Platform {
			case "youtube", "youtube_video":
				rep.YouTubeLinksFound++
				if targetAcc.PlatformID != "" && model.YouTubeID.MatchString(targetAcc.PlatformID) {
					rep.StableIDsResolved++
				} else if e.YouTubeResolver != nil {
					resolved, rerr := e.YouTubeResolver(ctx, *targetAcc)
					if rerr == nil && resolved.PlatformID != "" && model.YouTubeID.MatchString(resolved.PlatformID) {
						*targetAcc = resolved
						rep.StableIDsResolved++
					}
				}
			case "twitch":
				rep.TwitchLinksFound++
				if targetAcc.PlatformID != "" || targetAcc.Handle != "" {
					rep.StableIDsResolved++
				}
			case "x":
				rep.XLinksFound++
				if targetAcc.Handle != "" {
					rep.StableIDsResolved++
				}
			default:
				rep.OtherLinksFound++
			}

			targetKey := targetAcc.Key()
			isExisting := false
			if e.KnownCanonical != nil && (e.KnownCanonical[targetKey] != "" || e.KnownCanonical[targetAcc.Platform+":url:"+targetAcc.URL] != "") {
				isExisting = true
			}
			if !isExisting && e.KnownInbox != nil && (e.KnownInbox[targetKey] != "" || e.KnownInbox[targetAcc.Platform+":url:"+targetAcc.URL] != "") {
				isExisting = true
			}

			evidenceURL := "https://bsky.app/profile/" + c.PlatformID
			if c.Handle != "" {
				evidenceURL = "https://bsky.app/profile/" + c.Handle
			}

			if isExisting {
				rep.ExistingAccountsMatched++
			} else {
				if e.Store != nil {
					newCandAcc := *targetAcc
					if newCandAcc.Name == "" {
						newCandAcc.Name = prof.DisplayName
					}
					newCandAcc.ThaiRelevanceHint = "thai_signal"
					newCandAcc.ClassificationHint = "cross_platform_link_candidate"
					if c.Classification == "ORGANIZATION_SIGNAL" {
						newCandAcc.ClassificationHint = "ORGANIZATION_SIGNAL"
					}
					upserted, uerr := e.Store.Upsert(newCandAcc, evidenceURL, now)
					if uerr == nil && upserted.ID != "" {
						rep.NewAccountCandidates++
						if e.KnownInbox != nil {
							e.KnownInbox[targetKey] = upserted.ID
							if targetAcc.URL != "" {
								e.KnownInbox[targetAcc.Platform+":url:"+targetAcc.URL] = upserted.ID
							}
						}
					}
				}
			}

			// For YouTube, only propose if stable channel ID is known
			if targetAcc.Platform == "youtube" && !model.YouTubeID.MatchString(targetAcc.PlatformID) {
				continue
			}

			toPlatformID := targetAcc.PlatformID
			if toPlatformID == "" && targetAcc.Handle != "" {
				toPlatformID = targetAcc.Handle
			}

			proposal := model.RelationProposal{
				FromCandidateID: c.ID,
				FromPlatform:    "bluesky",
				FromPlatformID:  c.PlatformID,
				ToPlatform:      targetAcc.Platform,
				ToPlatformID:    toPlatformID,
				ToURL:           targetAcc.URL,
				EvidenceURL:     evidenceURL,
				EvidenceType:    "profile_description",
				Confidence:      conf,
				CreatedAt:       now.Format(time.RFC3339),
				ReviewStatus:    "pending_review",
			}

			if e.Store != nil {
				if perr := e.Store.AddRelationProposal(proposal); perr == nil {
					rep.RelationProposalsCount++
					rep.Proposals = append(rep.Proposals, proposal)
				}
			}
		}
	}
	return rep, nil
}
