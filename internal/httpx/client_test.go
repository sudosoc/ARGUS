// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseProxies(t *testing.T) {
	got, err := parseProxies([]string{" http://a:1 ", "", "socks5://b:2", "socks5h://c:3", "https://d:4"})
	if err != nil || len(got) != 4 {
		t.Fatalf("got %v %v", got, err)
	}
	for _, bad := range []string{"ftp://x:1", "nohost", "http://", "://x"} {
		if _, err := parseProxies([]string{bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestSpoofHeaders(t *testing.T) {
	h := http.Header{}
	spoofHeaders(h)
	for _, k := range []string{"User-Agent", "Accept", "Accept-Language", "Sec-Fetch-Mode"} {
		if h.Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
	if h.Get("Accept-Encoding") != "" {
		t.Error("Accept-Encoding must stay unset so Go decodes gzip itself")
	}
	if RandomUserAgent() == "" || randomAcceptLanguage() == "" {
		t.Error("empty random header")
	}
}

func newClient(t *testing.T, o Options) *Client {
	t.Helper()
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDoBasicAndBodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "1" || r.Header.Get("User-Agent") == "" {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("a", 5000)))
	}))
	defer srv.Close()
	c := newClient(t, Options{MaxBodyBytes: 1000})
	r, err := c.Do(context.Background(), Request{URL: srv.URL, Headers: map[string]string{"X-Test": "1"}})
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("%v %+v", err, r)
	}
	if len(r.Body) != 1000 {
		t.Errorf("body cap: got %d", len(r.Body))
	}
	r, _ = c.Do(context.Background(), Request{URL: srv.URL, Headers: map[string]string{"X-Test": "1"}, MaxBody: 10})
	if len(r.Body) != 10 {
		t.Errorf("per-request cap: got %d", len(r.Body))
	}
}

func TestDoRetriesTransient(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := newClient(t, Options{Retries: 2})
	r, err := c.Do(context.Background(), Request{URL: srv.URL})
	if err != nil || r.StatusCode != 200 || n.Load() != 2 {
		t.Fatalf("err=%v status=%d attempts=%d", err, r.StatusCode, n.Load())
	}
}

func TestDoNoRetryOnClientError(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := newClient(t, Options{Retries: 3})
	r, err := c.Do(context.Background(), Request{URL: srv.URL})
	if err != nil || r.StatusCode != 404 || n.Load() != 1 {
		t.Fatalf("err=%v attempts=%d", err, n.Load())
	}
}

func TestDoRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			_, _ = w.Write([]byte("done"))
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer srv.Close()
	c := newClient(t, Options{})
	r, err := c.Do(context.Background(), Request{URL: srv.URL + "/start"})
	if err != nil || r.StatusCode != 200 || !strings.HasSuffix(r.FinalURL, "/final") {
		t.Fatalf("follow: %v %+v", err, r)
	}
	r, err = c.Do(context.Background(), Request{URL: srv.URL + "/start", NoRedirect: true})
	if err != nil || r.StatusCode != 302 {
		t.Fatalf("no-follow: %v %+v", err, r)
	}
}

func TestDoContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) }))
	defer srv.Close()
	c := newClient(t, Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := c.Do(ctx, Request{URL: srv.URL}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRejectsBadProxy(t *testing.T) {
	if _, err := New(Options{Proxies: []string{"gopher://x:1"}}); err == nil {
		t.Fatal("expected error")
	}
}
