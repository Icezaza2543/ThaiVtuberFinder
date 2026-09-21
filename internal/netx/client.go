// Package netx provides bounded HTTP retries and public-only outbound connections.
package netx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	HTTP     *http.Client
	Attempts int
	Delay    time.Duration
	MaxBytes int64
}

func PublicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
func New() *Client {
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 16, MaxConnsPerHost: 4, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 20 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		if port != "443" && port != "80" {
			return nil, errors.New("non-standard destination port")
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, errors.New("destination lookup failed")
		}
		for _, ip := range ips {
			if !PublicIP(ip.IP) {
				return nil, errors.New("non-public destination blocked")
			}
		}
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("destination connection failed")
	}
	h := &http.Client{Timeout: 30 * time.Second, Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit")
		}
		if r.URL.Scheme != "https" || r.URL.User != nil {
			return errors.New("unsafe redirect")
		}
		if len(via) > 0 && r.URL.Hostname() != via[0].URL.Hostname() {
			r.Header.Del("Authorization")
			r.Header.Del("X-Goog-Api-Key")
		}
		return nil
	}}
	return &Client{HTTP: h, Attempts: 3, Delay: time.Second, MaxBytes: 16 << 20}
}
func (c *Client) JSON(ctx context.Context, method, raw string, headers map[string]string, body any, out any, before func() error) error {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	data, e := c.Bytes(ctx, method, raw, headers, b, before)
	if e != nil {
		return e
	}
	if out == nil {
		return nil
	}
	if e = json.Unmarshal(data, out); e != nil {
		return errors.New("invalid JSON response")
	}
	return nil
}
func (c *Client) Bytes(ctx context.Context, method, raw string, headers map[string]string, body []byte, before func() error) ([]byte, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid HTTP destination")
	}
	client := c.HTTP
	if client == nil {
		client = New().HTTP
	}
	attempts := c.Attempts
	if attempts < 1 {
		attempts = 1
	}
	maxBytes := c.MaxBytes
	if maxBytes < 1 {
		maxBytes = 16 << 20
	}
	delay := c.Delay
	if delay <= 0 {
		delay = time.Second
	}
	for i := 0; i < attempts; i++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if before != nil {
			if e = before(); e != nil {
				return nil, e
			}
		}
		req, e := http.NewRequestWithContext(ctx, method, raw, bytes.NewReader(body))
		if e != nil {
			return nil, errors.New("invalid HTTP request")
		}
		req.Header.Set("User-Agent", "ThaiVtuberFinder/0.1 (+https://github.com/Icezaza2543/ThaiVtuberFinder)")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		wait := delay * time.Duration(1<<i)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
			resp.Body.Close()
			if int64(len(data)) > maxBytes {
				return nil, errors.New("HTTP response exceeds size limit")
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if readErr != nil {
					return nil, errors.New("incomplete HTTP body")
				}
				return data, nil
			}
			if resp.StatusCode != 429 && resp.StatusCode < 500 {
				return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, u.Hostname())
			}
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, e := strconv.Atoi(ra); e == nil {
					wait = time.Duration(n) * time.Second
				} else if at, e := http.ParseTime(ra); e == nil {
					wait = time.Until(at)
				}
			}
			err = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u.Hostname())
		} else {
			err = fmt.Errorf("network request failed for %s", u.Hostname())
		}
		if i == attempts-1 {
			return nil, err
		}
		if wait > 60*time.Second {
			return nil, fmt.Errorf("server backoff exceeds retry budget for %s", u.Hostname())
		}
		if wait < 0 {
			wait = 0
		}
		wait += time.Duration(rand.IntN(20)) * time.Millisecond
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("request attempts exhausted")
}
func SafeSource(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return "invalid source"
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	return strings.TrimSpace(u.String())
}
