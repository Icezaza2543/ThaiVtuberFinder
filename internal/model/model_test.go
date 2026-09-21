package model

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, platform, id, handle, url string }{
		{"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa?x=1", "youtube", "UCaaaaaaaaaaaaaaaaaaaaaa", "", "https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa"},
		{"https://youtube.com/@SomeName/videos", "youtube", "", "@somename", "https://www.youtube.com/@somename"},
		{"https://twitter.com/EXAMPLE/status/123", "x", "", "example", "https://x.com/example"},
		{"https://twitch.tv/Example/about", "twitch", "", "example", "https://www.twitch.tv/example"},
		{"https://bsky.app/profile/did:plc:abcdef", "bluesky", "did:plc:abcdef", "", "https://bsky.app/profile/did:plc:abcdef"},
		{"https://youtu.be/abcdefghijk", "youtube_video", "abcdefghijk", "", "https://www.youtube.com/watch?v=abcdefghijk"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			a, err := Normalize(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if a.Platform != c.platform || a.PlatformID != c.id || a.Handle != c.handle || a.URL != c.url {
				t.Fatalf("got %+v", a)
			}
		})
	}
}
func TestNormalizeRejects(t *testing.T) {
	for _, raw := range []string{"javascript:alert(1)", "https://youtube.com.evil.test/@x", "https://youtube.com/channel/bad", "https://youtube.com/results?search_query=x", "https://youtube.com/", "https://user:pass@twitch.tv/example", "http://127.0.0.1/x", "https://twitch.tv/directory"} {
		if _, err := Normalize(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestKeysNeverUseDisplayName(t *testing.T) {
	a := Account{Platform: "youtube", PlatformID: "UCaaaaaaaaaaaaaaaaaaaaaa", Name: "Same"}
	b := Account{Platform: "youtube", PlatformID: "UCbbbbbbbbbbbbbbbbbbbbbb", Name: "Same"}
	if a.Key() == b.Key() {
		t.Fatal("name merged accounts")
	}
	a.Name = "Renamed"
	if a.Key() != "youtube:id:UCaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatal(a.Key())
	}
}
func TestClassifyIsSuggestionOnly(t *testing.T) {
	if Classify("Thai VTuber / Live2D", "Example") != "VTUBER_SIGNAL" {
		t.Fatal("signal missing")
	}
	if Classify("", "A cute character") != "UNRESOLVED" {
		t.Fatal("overconfident classifier")
	}
	if Classify("Official VTuber agency", "Team") != "ORGANIZATION_SIGNAL" {
		t.Fatal("agency not separated")
	}
}
