package netx

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRetryAndNoSecretErrors(t *testing.T) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()
	c := Client{HTTP: s.Client(), Attempts: 2, Delay: time.Millisecond}
	var v map[string]bool
	if e := c.JSON(context.Background(), "GET", s.URL+"?key=SECRET", nil, nil, &v, nil); e != nil || !v["ok"] || n != 2 {
		t.Fatalf("%v %v %d", e, v, n)
	}
}
func TestPermanentErrorNotRetried(t *testing.T) {
	n := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n++; w.WriteHeader(403); w.Write([]byte("key=SECRET")) }))
	defer s.Close()
	c := Client{HTTP: s.Client(), Attempts: 3}
	var v any
	e := c.JSON(context.Background(), "GET", s.URL+"?key=SECRET", nil, nil, &v, nil)
	if e == nil || strings.Contains(e.Error(), "SECRET") || n != 1 {
		t.Fatal(e, n)
	}
}
func TestRejectNonPublicIPs(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.1.1", "169.254.169.254", "::1", "fc00::1", "100.64.0.1"} {
		if PublicIP(net.ParseIP(s)) {
			t.Fatal(s)
		}
	}
	if !PublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public rejected")
	}
}
func TestBodyLimit(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 100))) }))
	defer s.Close()
	c := Client{HTTP: s.Client(), MaxBytes: 10}
	if _, e := c.Bytes(context.Background(), "GET", s.URL, nil, nil, nil); e == nil {
		t.Fatal("unbounded body")
	}
}
