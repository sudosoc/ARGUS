// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package posts copies what an account has published in public, from the four places that show it to anyone
// without an account: Mastodon, Bluesky, GitHub and Reddit. It never signs in and never uses a key or a cookie.
//
// It is opt-in: it runs only when named (the plan step in the web UI, or -modules on the command line), and only
// for the username and nickname the operator typed, never for guessed or followed ones. An account that asked not
// to be shown to people without an account is skipped, and so is one that cannot be read without signing in.
// Each source is limited to a small number of requests, spaced out, and stops at the first refusal.
package posts

import (
	"context"
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

const (
	maxPosts           = 150  // per account and place
	maxText            = 1200 // characters kept per item
	maxMedia           = 6    // pictures kept per item
	maxPictureEntities = 8    // pictures handed to the picture stage per account (the rest stay as addresses)
)

// pause is the least time between two requests to the same place (tests shorten it).
var pause = 1100 * time.Millisecond

// collector is one place to copy from.
type collector struct {
	module   string // module name, for example postsmastodon
	platform string
	desc     string
	// fetch copies the account's public items. note explains an empty or partial result.
	fetch func(ctx context.Context, c *httpx.Client, handle string) (items []models.Post, profileURL, note string)
}

// Scanner runs one collector for the typed usernames.
type Scanner struct {
	c collector
}

var _ scanner.Scanner = (*Scanner)(nil)

// OptIn marks the module as off unless it is named.
func (s *Scanner) OptIn() bool { return true }

func (s *Scanner) Name() string        { return s.c.module }
func (s *Scanner) Description() string { return s.c.desc }

// handles returns the usernames this module may use: only what the operator typed (username and nickname).
func handles(t models.Target) []string {
	var out []string
	seen := map[string]bool{}
	for _, h := range []string{t.Username, t.Alias} {
		h = strings.TrimPrefix(strings.TrimSpace(h), "@")
		if h == "" || strings.ContainsAny(h, " /\\") || strings.Count(h, "@") > 1 || seen[strings.ToLower(h)] {
			continue
		}
		seen[strings.ToLower(h)] = true
		out = append(out, h)
	}
	return out
}

func (s *Scanner) CanRun(t models.Target) bool {
	for _, h := range handles(t) {
		if s.c.platform == "Mastodon" || !strings.Contains(h, "@") { // user@host names an instance, so only Mastodon can use it
			return true
		}
	}
	return false
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	var wg sync.WaitGroup
	for _, h := range handles(t) {
		if s.c.platform != "Mastodon" && strings.Contains(h, "@") {
			continue
		}
		h := h
		wg.Add(1)
		if err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			items, profileURL, note := s.c.fetch(ctx, d.HTTP, h)
			if d.Progress != nil {
				d.Progress(s.Name(), 1, 1)
			}
			emit(finding(s.c, h, items, profileURL, note))
		}); err != nil {
			wg.Done()
			return err
		}
	}
	wg.Wait()
	return nil
}

// finding turns a collection into one finding that carries the items.
func finding(c collector, handle string, items []models.Post, profileURL, note string) models.Finding {
	src := &models.PostSource{Platform: c.platform, Handle: handle, URL: profileURL, Count: len(items), Note: note}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Time > items[j].Time })
	for _, p := range items {
		if p.Time == "" {
			continue
		}
		if src.Newest == "" || p.Time > src.Newest {
			src.Newest = p.Time
		}
		if src.Oldest == "" || p.Time < src.Oldest {
			src.Oldest = p.Time
		}
	}
	f := models.Finding{Source: c.platform + " public posts", Category: "posts", URL: profileURL, Confidence: 0.9, PostSource: src, Posts: items,
		Details: map[string]string{"account": handle}}
	switch {
	case len(items) > 0:
		f.Status = models.StatusFound
		f.Details["items copied"] = itoa(len(items))
		if src.Oldest != "" {
			f.Details["range"] = day(src.Oldest) + " → " + day(src.Newest)
		}
		if note != "" {
			f.Details["note"] = note
		}
		seen := map[string]bool{}
		for _, p := range items {
			for _, m := range p.Media {
				if len(f.Entities) >= maxPictureEntities {
					break
				}
				if !seen[m] {
					seen[m] = true
					f.Entities = append(f.Entities, models.Entity{Type: models.EntImage, Value: m})
				}
			}
		}
	case note != "":
		f.Status = models.StatusNotFound
		f.Details["note"] = note
	default:
		f.Status = models.StatusNotFound
		f.Details["note"] = "no public items were found"
	}
	// A refusal or a limit is a gap in coverage, not proof that nothing exists.
	if len(items) == 0 && (strings.Contains(note, "refused") || strings.Contains(note, "did not show") || strings.Contains(note, "stopped")) {
		f.Status = models.StatusError
		f.Details["error"] = note
	}
	return f
}

func day(t string) string {
	if len(t) >= 10 {
		return t[:10]
	}
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ------------------------------------------------------------------- shared helpers

var (
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	breakRe = regexp.MustCompile(`(?i)<br\s*/?>|</p>\s*<p[^>]*>`)
	hashRe  = regexp.MustCompile(`(?:^|[\s(])#([\p{L}\p{N}_]{2,64})`)
	urlRe   = regexp.MustCompile(`https?://[^\s<>"')\]]+`)
)

// plain turns the HTML Mastodon sends into text.
func plain(s string) string {
	s = breakRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// clip keeps the first max characters.
func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

// links returns the distinct web addresses in text.
func links(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, u := range urlRe.FindAllString(text, -1) {
		u = strings.TrimRight(u, ".,;:!?")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// hashtags returns the distinct hashtags in text, lower-cased.
func hashtags(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range hashRe.FindAllStringSubmatch(text, -1) {
		t := strings.ToLower(m[1])
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func uniq(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			out = append(out, s)
		}
	}
	return out
}

func utc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// spacer spaces a source's requests out, so nothing is requested faster than once a second.
type spacer struct {
	last time.Time
}

func (s *spacer) wait(ctx context.Context) bool {
	if !s.last.IsZero() {
		if d := pause - time.Since(s.last); d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return false
			}
		}
	}
	s.last = time.Now()
	return ctx.Err() == nil
}

// New returns every posts collector.
func New() []*Scanner {
	return []*Scanner{
		{c: collector{module: "postsmastodon", platform: "Mastodon", desc: "public posts of a Mastodon account, copied without signing in (opt-in)", fetch: mastodon}},
		{c: collector{module: "postsbluesky", platform: "Bluesky", desc: "public posts of a Bluesky account, copied without signing in (opt-in)", fetch: bluesky}},
		{c: collector{module: "postsgithub", platform: "GitHub", desc: "public activity and repositories of a GitHub account (opt-in)", fetch: github}},
		{c: collector{module: "postsreddit", platform: "Reddit", desc: "public posts and comments of a Reddit account, when Reddit shows them to people without an account (opt-in)", fetch: reddit}},
	}
}
