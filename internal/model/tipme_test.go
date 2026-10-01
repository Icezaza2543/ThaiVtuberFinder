package model

import "testing"

func TestTipmePublicCreatorURL(t *testing.T) {
	a, err := Normalize("https://www.tipme.in.th/Creator_One/?ref=bio#support")
	if err != nil || a.Platform != "tipme" || a.Handle != "creator_one" || a.URL != "https://tipme.in.th/creator_one" {
		t.Fatalf("unexpected creator URL: %+v, %v", a, err)
	}
	if a.PlatformID != "" {
		t.Fatal("mutable donation slug must not become a stable ID")
	}
	for _, raw := range []string{"https://tipme.in.th/", "https://tipme.in.th/login", "https://tipme.in.th/statistics", "https://tipme.in.th/dashboard", "https://tipme.in.th/discover", "https://tipme.in.th/api/creator", "https://tipme.in.th/creator/payment", "https://support.tipme.in.th/creator", "https://tipme.in.th.evil.test/creator"} {
		if _, err := Normalize(raw); err == nil {
			t.Errorf("accepted non-profile: %s", raw)
		}
	}
}
