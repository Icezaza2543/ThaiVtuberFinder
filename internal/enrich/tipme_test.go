package enrich

import "testing"

func TestExtractTipmeBioLink(t *testing.T) {
	links := ExtractLinks("Support: tipme.in.th/creator_one")
	if len(links) != 1 || links[0] != "https://tipme.in.th/creator_one" {
		t.Fatalf("missing explicit bio link: %v", links)
	}
	a, _, err := NormalizeLink(links[0])
	if err != nil || a.Platform != "tipme" || a.PlatformID != "" {
		t.Fatalf("unexpected Tipme account: %+v, %v", a, err)
	}
}
