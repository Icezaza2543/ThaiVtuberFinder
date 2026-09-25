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

func TestTwitchAccountNormalizationAndKey(t *testing.T) {
	// 1. twitch.tv/foo == www.twitch.tv/foo
	a1, err1 := Normalize("https://twitch.tv/foo")
	a2, err2 := Normalize("https://www.twitch.tv/foo")
	if err1 != nil || err2 != nil {
		t.Fatalf("Normalize error: %v, %v", err1, err2)
	}
	if a1.Key() != a2.Key() || a1.URL != a2.URL {
		t.Fatalf("twitch.tv vs www.twitch.tv mismatch: a1=%+v, a2=%+v", a1, a2)
	}

	// 2. Twitch.TV/Foo/ == twitch.tv/foo
	a3, err3 := Normalize("https://Twitch.TV/Foo/")
	if err3 != nil {
		t.Fatalf("Normalize error for Twitch.TV/Foo/: %v", err3)
	}
	if a3.Key() != a1.Key() || a3.URL != a1.URL {
		t.Fatalf("Twitch.TV/Foo/ mismatch: got key=%q, want key=%q", a3.Key(), a1.Key())
	}

	// 3. query string does not create a new account
	a4, err4 := Normalize("https://www.twitch.tv/foo?ref=social&campaign=1#live")
	if err4 != nil {
		t.Fatalf("Normalize error for query string URL: %v", err4)
	}
	if a4.Key() != a1.Key() || a4.URL != a1.URL {
		t.Fatalf("query string created different account: got key=%q, want key=%q", a4.Key(), a1.Key())
	}

	// 4. display_name change does not create a new candidate key
	accName1 := Account{Platform: "twitch", Handle: "foo", Name: "Foo Original", URL: "https://www.twitch.tv/foo"}
	accName2 := Account{Platform: "twitch", Handle: "foo", Name: "Foo Completely Renamed", URL: "https://www.twitch.tv/foo"}
	if accName1.Key() != accName2.Key() {
		t.Fatalf("display_name change created different key: %q != %q", accName1.Key(), accName2.Key())
	}

	// Stable platform ID wins over URL/handle
	accWithID := Account{Platform: "twitch", PlatformID: "123456", Handle: "foo", URL: "https://www.twitch.tv/foo"}
	if accWithID.Key() != "twitch:id:123456" {
		t.Fatalf("stable platform ID did not win: got %s", accWithID.Key())
	}
}
