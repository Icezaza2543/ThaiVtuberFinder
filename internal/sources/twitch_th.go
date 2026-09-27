package sources

import (
	"context"
	"errors"
	"os"

	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/twitch"
)

// twitchThai samples Thai-language Twitch broadcasters with a virtual-creator
// signal (live streams + channel search) via Helix. Each cycle is a snapshot, so
// coverage grows over time. Logins only (numeric IDs are resolved at review), to
// keep dedupe keys consistent with existing Twitch inbox rows.
func (e *Engine) twitchThai(ctx context.Context, c Config, limit int) ([]Lead, error) {
	id, secret := os.Getenv("TWITCH_CLIENT_ID"), os.Getenv("TWITCH_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil, errors.New("twitch_th requires TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET")
	}
	pages := c.MaxPages
	if pages <= 0 {
		pages = 20
	}
	r := &twitch.Resolver{ClientID: id, ClientSecret: secret}
	channels, err := r.ThaiVTubers(ctx, []string{"vtuber", "vtuberth", "vtuber th", "วีทูป", "pngtuber"}, pages)
	if err != nil {
		return nil, err
	}
	out := []Lead{}
	for _, ch := range channels {
		a, err := model.Normalize("https://www.twitch.tv/" + ch.Login)
		if err != nil {
			continue
		}
		a.Name = ch.DisplayName
		a.ClassificationHint = "VTUBER_SIGNAL"
		out = append(out, Lead{a, "https://www.twitch.tv/" + ch.Login})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
