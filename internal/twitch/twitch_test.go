package twitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin(t *testing.T) {
	cases := map[string]string{"https://www.twitch.tv/NisaPoyo": "nisapoyo", "@Abc_1": "abc_1", "https://x.com/abc": "", "https://www.twitch.tv/": ""}
	for in, want := range cases {
		got, ok := Login(in)
		if ok != (want != "") || (ok && got != want) {
			t.Errorf("%s: got %q ok=%v want %q", in, got, ok, want)
		}
	}
}

func TestResolve(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			w.Write([]byte(`{"access_token":"tok"}`))
		case "/helix/users":
			if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("Client-Id") != "cid" {
				w.WriteHeader(401)
				return
			}
			w.Write([]byte(`{"data":[{"id":"123","login":"alive","display_name":"Alive"}]}`))
		}
	}))
	defer srv.Close()
	r := &Resolver{ClientID: "cid", ClientSecret: "sec", AuthBase: srv.URL, APIBase: srv.URL + "/helix"}
	got, err := r.Resolve(context.Background(), []string{"alive", "gone"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].UserID != "123" || got[0].Status != "resolved" || got[1].Status != "not_found" || got[1].UserID != "" {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestResolveRequiresCredentials(t *testing.T) {
	if _, err := (&Resolver{}).Resolve(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected error")
	}
}
