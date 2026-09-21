package sheets

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/model"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testToken string

func (t testToken) Token(context.Context) (string, error) { return string(t), nil }
func TestJWTSignature(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	s := ServiceAccount{email: "test@test.iam.gserviceaccount.com", key: key}
	jwt, e := s.assertion(time.Now())
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatal("JWT parts")
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if e = rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig); e != nil {
		t.Fatal(e)
	}
	body, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	json.Unmarshal(body, &claims)
	if claims["aud"] != "https://oauth2.googleapis.com/token" {
		t.Fatal("wrong audience")
	}
}
func TestSyncRepeatedRunDoesNotAppendOrOverwriteReview(t *testing.T) {
	inbox := [][]string{append([]string{}, InboxHeaders...)}
	writes := 0
	sv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("auth missing")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/values:batchGet"):
			json.NewEncoder(w).Encode(map[string]any{"valueRanges": []any{map[string]any{"values": inbox}, map[string]any{"values": [][]string{{"account_id", "platform", "platform_id", "canonical_url"}}}, map[string]any{"values": [][]string{{"account_id", "persona_id", "review_status", "valid_to"}}}, map[string]any{"values": [][]string{{"persona_id", "review_status"}}}}})
		case strings.HasSuffix(r.URL.Path, "/values:batchUpdate"):
			writes++
			var body struct {
				Option string   `json:"valueInputOption"`
				Data   []Update `json:"data"`
			}
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
				t.Error(e)
			}
			if body.Option != "RAW" {
				t.Error("formula injection exposure")
			}
			for _, u := range body.Data {
				switch u.Range {
				case "'FINDER_INBOX'!A2:T2":
					if len(inbox) != 1 {
						t.Error("duplicate append")
					}
					inbox = append(inbox, u.Values[0])
				case "'FINDER_INBOX'!A2:K2":
					copy(inbox[1][:11], u.Values[0])
				case "'FINDER_INBOX'!T2":
					inbox[1][19] = u.Values[0][0]
				default:
					t.Errorf("unexpected update %s", u.Range)
				}
			}
			w.Write([]byte(`{}`))
		default:
			w.Write([]byte(`{"sheets":[{"properties":{"sheetId":1,"title":"FINDER_INBOX","gridProperties":{"rowCount":1001,"columnCount":20}}}]}`))
		}
	}))
	defer sv.Close()
	cl := Client{HTTP: &netx.Client{HTTP: sv.Client()}, Tokens: testToken("test"), SpreadsheetID: "example", BaseURL: sv.URL + "/"}
	c := candidate()
	c.Name = "=IMPORTXML(\"bad\")"
	if _, e := cl.Sync(context.Background(), []model.Candidate{c}); e != nil {
		t.Fatal(e)
	}
	inbox[1][11] = "verified"
	inbox[1][16] = "owner"
	c.Name = "Renamed"
	if _, e := cl.Sync(context.Background(), []model.Candidate{c}); e != nil {
		t.Fatal(e)
	}
	if len(inbox) != 2 || inbox[1][11] != "verified" || inbox[1][16] != "owner" || writes != 2 {
		t.Fatal("sync did not preserve curator state", inbox)
	}
}
func TestCanonicalBootstrapShape(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bootstrap.json")
	os.WriteFile(p, []byte(`{"tables":{"accounts":[{"id":"acct_1","platform":"youtube","platform_id":"UCaaaaaaaaaaaaaaaaaaaaaa","url":"https://www.youtube.com/channel/UCaaaaaaaaaaaaaaaaaaaaaa"}],"personas":[{"id":"persona_1","review_status":"verified"}],"account_links":[{"account_id":"acct_1","persona_id":"persona_1","review_status":"verified","valid_to":null}]}}`), 0600)
	known, pids, e := LoadCanonical(p)
	if e != nil || !pids["persona_1"] || known["youtube:id:UCaaaaaaaaaaaaaaaaaaaaaa"] != "KNOWN_LINKED" {
		t.Fatal(e, known, pids)
	}
}
