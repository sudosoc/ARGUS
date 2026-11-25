// Package social implements the username-enumeration module: it probes a
// large list of platforms concurrently and decides, from the HTTP status
// code and page content, whether the account exists.
package social

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sudosoc/argus/internal/extract"
	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

// Config tunes the module.
type Config struct {
	SitesFile  string   // custom site list (WMN format); empty = embedded defaults
	Categories []string // restrict to these site categories; empty = all
	Verify     bool     // re-probe each hit with a random canary username to reject false positives
	// DisabledFile lists sites a self-test found unreliable; they are skipped.
	// Empty means use every site.
	DisabledFile string
	// Tier bounds how deep into the site list a scan goes: 1 core, 2 extended
	// (default when 0), 3 everything including template-based forums.
	Tier int
}

// Scanner is the social-media username module.
type Scanner struct {
	cfg   Config
	sites []Site
}

var _ scanner.Scanner = (*Scanner)(nil)

// New loads the site list and returns a ready module.
func New(cfg Config) (*Scanner, error) {
	sites, err := LoadSites(cfg.SitesFile)
	if err != nil {
		return nil, err
	}
	if off := LoadDisabled(cfg.DisabledFile); len(off) > 0 {
		kept := sites[:0]
		for _, st := range sites {
			if !off[st.Name] {
				kept = append(kept, st)
			}
		}
		sites = kept
	}
	maxTier := cfg.Tier
	if maxTier <= 0 {
		maxTier = TierExtended
	}
	{
		kept := sites[:0]
		for _, st := range sites {
			if st.tier() <= maxTier {
				kept = append(kept, st)
			}
		}
		sites = kept
	}
	if len(cfg.Categories) > 0 {
		want := map[string]bool{}
		for _, c := range cfg.Categories {
			want[strings.ToLower(strings.TrimSpace(c))] = true
		}
		filtered := sites[:0]
		for _, s := range sites {
			cat := strings.ToLower(s.Cat)
			match := want[cat] || (want["nsfw"] && isNSFW(cat))
			for _, tg := range s.Tags {
				match = match || want[strings.ToLower(tg)]
			}
			if match {
				filtered = append(filtered, s)
			}
		}
		sites = filtered
	} else {
		// Adult sites are opt-in (-categories nsfw): they add noise and are
		// rarely what an investigation needs.
		filtered := sites[:0]
		for _, s := range sites {
			if !isNSFW(strings.ToLower(s.Cat)) {
				filtered = append(filtered, s)
			}
		}
		sites = filtered
	}
	return &Scanner{cfg: cfg, sites: sites}, nil
}

// SiteCount returns how many platforms are loaded.
func (s *Scanner) SiteCount() int { return len(s.sites) }

func (s *Scanner) Name() string                { return "social" }
func (s *Scanner) Description() string         { return "username presence across social/dev/media platforms" }
func (s *Scanner) CanRun(t models.Target) bool { return len(t.Usernames()) > 0 }

// Scan fans one task per (username, site) pair out over the shared pool.
func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	users := t.Usernames()
	total := len(users) * len(s.sites)

	var (
		wg      sync.WaitGroup
		done    atomic.Int64
		retryMu sync.Mutex
		retry   []retryJob
		tblMu   sync.Mutex
		table   []string // every check and its outcome: nothing is silent
	)
	rec := func(site *Site, user, outcome string, code int, link string) {
		tblMu.Lock()
		table = append(table, site.Name+"\t"+user+"\t"+outcome+"\t"+strconv.Itoa(code)+"\t"+link)
		tblMu.Unlock()
	}
	report := func() {
		n := int(done.Add(1))
		if d.Progress != nil {
			d.Progress(s.Name(), n, total)
		}
	}

submit:
	for _, user := range users {
		for i := range s.sites {
			site := &s.sites[i]
			wg.Add(1)
			err := d.Pool.Submit(ctx, func() {
				defer wg.Done()
				defer report()
				if s.check(ctx, d.HTTP, site, user, emit, false, rec) {
					retryMu.Lock()
					retry = append(retry, retryJob{site, user})
					retryMu.Unlock()
				}
			})
			if err != nil { // context cancelled: task was not queued
				wg.Done()
				break submit
			}
		}
	}
	wg.Wait()

	// Second pass: connection-level failures (timeouts, DNS) on a busy or
	// congested link are usually transient. Retry just those, gently, and only
	// report an error if they fail again.
	if len(retry) > 0 && ctx.Err() == nil {
		sem := make(chan struct{}, 12)
		for _, j := range retry {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer func() { <-sem; wg.Done() }()
				s.check(ctx, d.HTTP, j.site, j.user, emit, true, rec)
			}()
		}
		wg.Wait()
	}

	// The full table of what was checked and what came back (found, not found,
	// blocked, inconclusive, failed), kept whole in the report.
	if len(table) > 0 {
		sort.Strings(table)
		out := "site\tusername\toutcome\thttp status\turl\n" + strings.Join(table, "\n")
		emit(models.Finding{Source: "social: every check", Category: "toolrun", Status: models.StatusFound, Details: map[string]string{
			"command": strconv.Itoa(len(table)) + " site checks", "output": out, "bytes": strconv.Itoa(len(out)), "truncated": "no"}})
	}
	return ctx.Err()
}

type retryJob struct {
	site *Site
	user string
}

type verdict int

const (
	vAbsent verdict = iota
	vPresent
	vBlocked // rate-limited / WAF / captcha: result untrustworthy
	vUnknown // response matched neither the "exists" nor the "missing" signature
)

// check probes one site for one username and emits the outcome. When final is
// false, a connection-level failure is not reported: check returns true so the
// caller can retry it later.
func (s *Scanner) check(ctx context.Context, c *httpx.Client, site *Site, user string, emit func(models.Finding), final bool, rec func(*Site, string, string, int, string)) (retry bool) {
	if ctx.Err() != nil || !site.applicable(user) {
		return false
	}
	base := models.Finding{Source: site.Name, Category: site.Cat, URL: site.prettyURL(user)}

	v, resp, err := probe(ctx, c, site, user)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		if !final {
			return true
		}
		f := base
		f.Status = models.StatusError
		f.Details = map[string]string{"error": err.Error()}
		rec(site, user, "failed: "+err.Error(), 0, base.URL)
		emit(f)
		return false
	}

	switch v {
	case vPresent:
		conf := 0.6 // status code alone
		if site.hasMarker() {
			conf = 0.9 // positive content marker as well
		}
		if s.cfg.Verify {
			canary := randomCanary()
			cv, _, cerr := probe(ctx, c, site, canary)
			switch {
			case ctx.Err() != nil:
				return false
			case cerr == nil && cv == vPresent:
				// The site says a random string exists too: signature is useless here.
				f := base
				f.Status = models.StatusNotFound
				f.Details = map[string]string{"reason": "canary username also matched (false positive)"}
				rec(site, user, "rejected: a random username also matched", resp.StatusCode, base.URL)
				emit(f)
				return false
			case cerr == nil && cv == vAbsent:
				conf = minF(conf+0.05, 0.95)
			}
		}
		f := base
		f.Status = models.StatusFound
		f.Confidence = conf
		f.Details = map[string]string{"username": user, "http_status": strconv.Itoa(resp.StatusCode)}
		// Profile pages usually carry Open Graph metadata: the display name,
		// bio and avatar are useful for matching the account to the subject.
		if m := extract.ParseMeta(string(resp.Body)); m.Title != "" || m.Description != "" {
			if t := util.FirstNonEmpty(m.OGTitle, m.Title); t != "" && !strings.EqualFold(t, site.Name) {
				f.Details["page title"] = util.Clip(t, 140)
			}
			if m.Description != "" {
				f.Details["description"] = util.Clip(m.Description, 220)
			}
			if m.OGImage != "" {
				f.Details["avatar"] = util.Clip(m.OGImage, 300)
			}
		}
		rec(site, user, "found", resp.StatusCode, base.URL)
		emit(f)
	case vBlocked:
		f := base
		f.Status = models.StatusError
		f.Details = map[string]string{"error": "blocked or rate-limited", "http_status": strconv.Itoa(resp.StatusCode)}
		rec(site, user, "blocked or rate-limited", resp.StatusCode, base.URL)
		emit(f)
	case vUnknown:
		f := base
		f.Status = models.StatusUncertain
		f.Confidence = 0.3
		f.Details = map[string]string{"username": user, "http_status": strconv.Itoa(resp.StatusCode), "note": "response matched no known signature"}
		// Only surfaced when the status is a success code; other statuses are noise.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			emit(f)
		}
		rec(site, user, "inconclusive: the response matched no signature", resp.StatusCode, base.URL)
	default:
		f := base
		f.Status = models.StatusNotFound
		rec(site, user, "not found", resp.StatusCode, base.URL)
		emit(f)
	}
	return false
}

// probe sends the request for one (site, username) and classifies the reply.
func probe(ctx context.Context, c *httpx.Client, site *Site, user string) (verdict, *httpx.Response, error) {
	enc := url.PathEscape(site.clean(user))
	req := httpx.Request{
		Method:     "GET",
		URL:        site.checkURL(enc),
		Headers:    site.Headers,
		NoRedirect: site.Check == checkRedirect,
	}
	if site.PostBody != "" {
		req.Method = "POST"
		req.Body = strings.ReplaceAll(site.PostBody, accountToken, user)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return vUnknown, nil, err
	}
	return classify(site, resp), resp, nil
}

// classify applies the site's signatures to a response.
func classify(site *Site, r *httpx.Response) verdict {
	body := string(r.Body)
	code := r.StatusCode
	for _, e := range site.Errors {
		if e != "" && strings.Contains(body, e) {
			return vBlocked
		}
	}
	if site.Ignore403 && code == 403 {
		code = site.ECode
	}

	if site.Check == checkRedirect {
		switch {
		case code >= 200 && code < 300:
			return vPresent
		case code == 403 || code == 407 || code == 429 || code == 503:
			return vBlocked
		case code >= 300 && code < 500:
			return vAbsent
		}
		return vUnknown
	}

	// "Missing" signatures win: many sites return 200 with a "not found" page.
	if site.MString != "" && strings.Contains(body, site.MString) {
		return vAbsent
	}
	for _, m := range site.MStrings {
		if m != "" && strings.Contains(body, m) {
			return vAbsent
		}
	}
	if site.MCode != 0 && code == site.MCode && site.MString == "" && len(site.MStrings) == 0 {
		return vAbsent
	}
	if code == site.ECode && site.hasExistsMarker(body) {
		return vPresent
	}

	switch code {
	case 403, 407, 429, 503:
		return vBlocked
	case 404, 410:
		return vAbsent
	}
	// Status matched "exists" but the content marker did not: the page is a
	// generic/soft-404 shell rather than a profile.
	if code == site.ECode {
		return vAbsent
	}
	return vUnknown
}

// hasMarker reports whether the site defines any positive content marker.
func (s *Site) hasMarker() bool { return s.EString != "" || len(s.EStrings) > 0 }

// hasExistsMarker is true when the site needs no positive marker or the body has one.
func (s *Site) hasExistsMarker(body string) bool {
	if !s.hasMarker() {
		return true
	}
	if s.EString != "" && strings.Contains(body, s.EString) {
		return true
	}
	for _, m := range s.EStrings {
		if m != "" && strings.Contains(body, m) {
			return true
		}
	}
	return false
}

func randomCanary() string {
	b := make([]byte, 7)
	_, _ = rand.Read(b)
	return "zq" + hex.EncodeToString(b) // 16 chars, letters+digits, valid in subdomains
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func isNSFW(cat string) bool { return strings.Contains(cat, "nsfw") }
