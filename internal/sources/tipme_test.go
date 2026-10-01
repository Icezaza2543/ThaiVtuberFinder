package sources

import (
	"strings"
	"testing"
)

func TestTipmeImportPreservesStableCreatorEvidence(t *testing.T) {
	rows := `{"url":"https://tipme.in.th/creator_one","source_url":"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa","description":"Official public VTuberTH channel description link"}`
	leads, err := ParseJSONL(strings.NewReader(rows), 10)
	if err != nil || len(leads) != 1 {
		t.Fatalf("import: %v, %v", leads, err)
	}
	if leads[0].SourceURL != "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa" || leads[0].Account.Platform != "tipme" || leads[0].Account.PlatformID != "" {
		t.Fatalf("lost evidence or invented stable identity: %+v", leads[0])
	}
}
