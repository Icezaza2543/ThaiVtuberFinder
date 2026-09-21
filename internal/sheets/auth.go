package sheets

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"github.com/Icezaza2543/ThaiVtuberFinder/internal/netx"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type TokenSource interface {
	Token(context.Context) (string, error)
}
type ServiceAccount struct {
	mu     sync.Mutex
	email  string
	key    *rsa.PrivateKey
	http   *netx.Client
	token  string
	expiry time.Time
}

func Credentials(client *netx.Client) (TokenSource, error) {
	data := []byte(os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"))
	if len(data) == 0 {
		path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if path == "" {
			return nil, ErrNotConfigured
		}
		var e error
		data, e = os.ReadFile(path)
		if e != nil {
			return nil, errors.New("cannot read Google credential file")
		}
	}
	return parseCredentials(data, client)
}
func parseCredentials(data []byte, client *netx.Client) (*ServiceAccount, error) {
	var doc struct {
		Type     string `json:"type"`
		Email    string `json:"client_email"`
		Key      string `json:"private_key"`
		TokenURI string `json:"token_uri"`
	}
	if e := json.Unmarshal(data, &doc); e != nil || doc.Type != "service_account" || !strings.HasSuffix(doc.Email, ".iam.gserviceaccount.com") {
		return nil, errors.New("valid Google service-account credentials required")
	}
	if doc.TokenURI != "" && doc.TokenURI != "https://oauth2.googleapis.com/token" {
		return nil, errors.New("unexpected service-account token endpoint")
	}
	block, _ := pem.Decode([]byte(doc.Key))
	if block == nil {
		return nil, errors.New("invalid service-account private key")
	}
	key, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		return nil, errors.New("invalid PKCS8 private key")
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok || rsaKey.N.BitLen() < 2048 {
		return nil, errors.New("RSA key of at least 2048 bits required")
	}
	return &ServiceAccount{email: doc.Email, key: rsaKey, http: client}, nil
}
func (s *ServiceAccount) assertion(now time.Time) (string, error) {
	enc := base64.RawURLEncoding.EncodeToString
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": s.email, "scope": "https://www.googleapis.com/auth/spreadsheets", "aud": "https://oauth2.googleapis.com/token", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()})
	unsigned := enc(h) + "." + enc(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, e := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if e != nil {
		return "", errors.New("cannot sign service-account assertion")
	}
	return unsigned + "." + enc(sig), nil
}
func (s *ServiceAccount) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Until(s.expiry) > time.Minute {
		return s.token, nil
	}
	jwt, e := s.assertion(time.Now())
	if e != nil {
		return "", e
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {jwt}}
	b, e := s.http.Bytes(ctx, "POST", "https://oauth2.googleapis.com/token", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte(form.Encode()), nil)
	if e != nil {
		return "", e
	}
	var r struct {
		Token   string `json:"access_token"`
		Expires int    `json:"expires_in"`
	}
	if json.Unmarshal(b, &r) != nil || r.Token == "" || r.Expires <= 0 {
		return "", errors.New("invalid OAuth token response")
	}
	s.token = r.Token
	s.expiry = time.Now().Add(time.Duration(r.Expires) * time.Second)
	return s.token, nil
}
