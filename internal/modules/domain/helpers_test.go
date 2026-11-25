// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestItoa(t *testing.T) {
	for n, want := range map[int]string{0: "0", 7: "7", 120: "120", -45: "-45", 1234567890: "1234567890"} {
		if got := strconv.Itoa(n); got != want {
			t.Errorf("strconv.Itoa(%d) = %q", n, got)
		}
	}
}

func TestList(t *testing.T) {
	if got := list([]string{"a", "b"}, 5); got != "a, b" {
		t.Errorf("got %q", got)
	}
	if got := list([]string{"a", "b", "c", "d"}, 2); got != "a, b … (+2 more)" {
		t.Errorf("got %q", got)
	}
}

func TestSameSite(t *testing.T) {
	cases := []struct {
		host, dom string
		want      bool
	}{
		{"example.com", "example.com", true}, {"WWW.Example.com.", "example.com", true},
		{"a.b.example.com", "example.com", true}, {"notexample.com", "example.com", false},
		{"example.com.evil.net", "example.com", false},
	}
	for _, c := range cases {
		if got := sameSite(c.host, c.dom); got != c.want {
			t.Errorf("sameSite(%q,%q) = %v", c.host, c.dom, got)
		}
	}
}

func TestHumanAge(t *testing.T) {
	day := 24 * time.Hour
	for d, want := range map[time.Duration]string{10 * day: "10 days", 90 * day: "3 months", 800 * day: "2 years"} {
		if got := humanAge(d); got != want {
			t.Errorf("humanAge(%v) = %q want %q", d, got, want)
		}
	}
}

func TestCookieSummary(t *testing.T) {
	h := http.Header{}
	h.Add("Set-Cookie", "sid=1; Secure; HttpOnly")
	h.Add("Set-Cookie", "track=2")
	got := cookieSummary(h)
	if !strings.HasPrefix(got, "sid; track (no-Secure,no-HttpOnly)") {
		t.Errorf("got %q", got)
	}
}
