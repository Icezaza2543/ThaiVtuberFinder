package sources

import "testing"

func TestParseTipjaiDirectory(t *testing.T) {
	body := `<a href="/abel_vt">A</a><a href="/discover">d</a><a href="/Grazia_Vtuber">G</a><a href="/abel_vt">dup</a><a href="/apple-icon.png">x</a>`
	got := ParseTipjaiDirectory(body)
	if len(got) != 2 || got[0] != "abel_vt" || got[1] != "grazia_vtuber" {
		t.Fatalf("got %v", got)
	}
}

func TestParseTipjaiCreatorSkipsSiteAccountsAndPayment(t *testing.T) {
	body := `<title>โดเนท Abel (@abel_vt) — ทิปขึ้นจอ</title>
	<a href="https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa">yt</a>
	<a href="https://x.com/Abel_VT">x</a>
	<a href="https://x.com/Tipjai_official">site</a>
	<a href="https://www.tiktok.com/@tipjai.official">site</a>
	<a href="https://www.instagram.com/pang.42_?ig">footer</a>
	<span>พร้อมเพย์ 081-234-5678</span>`
	site := SiteSocialLinks(`<footer><a href="https://www.instagram.com/pang.42_?ig">ig</a></footer>`)
	leads := ParseTipjaiCreator(body, "abel_vt", "https://tipjai.com/abel_vt", site)
	if len(leads) != 3 {
		t.Fatalf("want tipjai+youtube+x, got %d: %+v", len(leads), leads)
	}
	if leads[0].Account.Platform != "tipjai" || leads[0].Account.Name != "Abel" {
		t.Fatalf("bad tipjai lead %+v", leads[0].Account)
	}
	for _, l := range leads {
		if l.Account.Handle == "tipjai_official" || l.Account.Handle == "@tipjai.official" {
			t.Fatalf("site account leaked: %+v", l.Account)
		}
	}
}

func TestParseTipjaiCreatorRejectsSitePages(t *testing.T) {
	if got := ParseTipjaiCreator(`<title>Tipjai Studio</title>`, "studio", "https://tipjai.com/studio", nil); got != nil {
		t.Fatalf("site page produced leads: %+v", got)
	}
}
