// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package social

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func resp(code int, body string) *httpx.Response {
	return &httpx.Response{StatusCode: code, Body: []byte(body)}
}

func TestClassify(t *testing.T) {
	site := &Site{ECode: 200, EString: "profile-of", MCode: 404, MString: "no such user"}
	cases := []struct {
		name string
		r    *httpx.Response
		want verdict
	}{
		{"exists", resp(200, "<h1>profile-of bob</h1>"), vPresent},
		{"missing marker wins over 200", resp(200, "no such user profile-of"), vAbsent},
		{"soft 404 shell", resp(200, "<html>generic</html>"), vAbsent},
		{"404", resp(404, ""), vAbsent},
		{"410", resp(410, ""), vAbsent},
		{"rate limited", resp(429, ""), vBlocked},
		{"waf", resp(403, ""), vBlocked},
		{"unavailable", resp(503, ""), vBlocked},
		{"odd status", resp(418, ""), vUnknown},
	}
	for _, c := range cases {
		if got := classify(site, c.r); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestClassifyStatusOnlySite(t *testing.T) {
	site := &Site{ECode: 200, MCode: 404}
	if classify(site, resp(200, "anything")) != vPresent {
		t.Error("200 with no marker should be present")
	}
	if classify(site, resp(404, "")) != vAbsent {
		t.Error("404 should be absent")
	}
}

func TestSiteURLs(t *testing.T) {
	s := &Site{URICheck: "https://x.example/api/{account}", URIPretty: "https://x.example/@{account}"}
	if got := s.checkURL("bob"); got != "https://x.example/api/bob" {
		t.Errorf("checkURL: %s", got)
	}
	if got := s.prettyURL("bob"); got != "https://x.example/@bob" {
		t.Errorf("prettyURL: %s", got)
	}
	s.URIPretty = ""
	if got := s.prettyURL("bob"); got != "https://x.example/api/bob" {
		t.Errorf("prettyURL fallback: %s", got)
	}
}

func TestApplicable(t *testing.T) {
	s := &Site{subdomain: true}
	if !s.applicable("bob-1") || s.applicable("john.doe") || s.applicable("a/b") {
		t.Error("subdomain sites must reject dots and slashes")
	}
	if !(&Site{}).applicable("john.doe") {
		t.Error("path sites accept dots")
	}
}

func TestLoadSitesEmbedded(t *testing.T) {
	sites, err := LoadSites("")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) < 5000 {
		t.Fatalf("embedded list suspiciously small: %d", len(sites))
	}
	seen := map[string]bool{}
	tiers := map[int]int{}
	for _, s := range sites {
		tiers[s.tier()]++
		if s.Name == "" || s.URICheck == "" {
			t.Fatalf("site without name or URL: %+v", s)
		}
		if !contains(s.URICheck+s.PostBody, "{account}") {
			t.Errorf("%s: neither uri_check nor post_body has {account}", s.Name)
		}
		if seen[s.Name] {
			t.Errorf("duplicate site name %q", s.Name)
		}
		seen[s.Name] = true
	}
	if tiers[TierCore] < 1000 || tiers[TierExtended] < 1500 || tiers[TierAll] < 5000 {
		t.Errorf("tier split looks wrong: %v", tiers)
	}
}

func TestTierFilter(t *testing.T) {
	core, _ := New(Config{Tier: TierCore})
	ext, _ := New(Config{})
	all, _ := New(Config{Tier: TierAll})
	if !(core.SiteCount() < ext.SiteCount() && ext.SiteCount() < all.SiteCount()) {
		t.Errorf("tiers must nest: %d < %d < %d", core.SiteCount(), ext.SiteCount(), all.SiteCount())
	}
}

func TestClassifyMultiMarker(t *testing.T) {
	site := &Site{ECode: 200, EStrings: []string{"profilePage", "memberHeader"}, MStrings: []string{"not found", "no such member"}}
	cases := map[string]struct {
		r    *httpx.Response
		want verdict
	}{
		"present marker 2": {resp(200, "<div class='memberHeader'>"), vPresent},
		"absent marker 2":  {resp(200, "Sorry, no such member"), vAbsent},
		"neither":          {resp(200, "<html>shell</html>"), vAbsent},
	}
	for n, c := range cases {
		if got := classify(site, c.r); got != c.want {
			t.Errorf("%s: got %v want %v", n, got, c.want)
		}
	}
}

func TestClassifyBlockedErrorsAndIgnore403(t *testing.T) {
	site := &Site{ECode: 200, Errors: []string{"captcha required"}}
	if classify(site, resp(200, "please solve: captcha required")) != vBlocked {
		t.Error("error marker must classify as blocked")
	}
	f := &Site{ECode: 200, EStrings: []string{"profilePage"}, Ignore403: true}
	if classify(f, resp(403, "profilePage")) != vPresent || classify(f, resp(403, "denied")) != vAbsent {
		t.Error("ignore403 must judge a 403 page by its content")
	}
}

func TestClassifyRedirectCheck(t *testing.T) {
	site := &Site{Check: checkRedirect}
	for code, want := range map[int]verdict{200: vPresent, 302: vAbsent, 404: vAbsent, 429: vBlocked, 500: vUnknown} {
		if got := classify(site, resp(code, "")); got != want {
			t.Errorf("redirect check %d: got %v want %v", code, got, want)
		}
	}
}

func TestRegexAndStrip(t *testing.T) {
	s := &Site{Regex: `^[a-z0-9]{3,10}$`, Strip: ".-"}
	_ = s.compile()
	if !s.applicable("ab-c.d") || s.applicable("ab") || s.applicable("UPPER") {
		t.Error("regex is checked after stripping disallowed characters")
	}
	if got := s.clean("a.b-c"); got != "abc" {
		t.Errorf("clean: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLoadSitesFormats(t *testing.T) {
	dir := t.TempDir()
	obj := filepath.Join(dir, "obj.json")
	arr := filepath.Join(dir, "arr.json")
	_ = os.WriteFile(obj, []byte(`{"sites":[{"name":"A","uri_check":"https://a/{account}","e_code":200,"m_code":404}]}`), 0o600)
	_ = os.WriteFile(arr, []byte(`[{"name":"B","uri_check":"https://b/{account}"}]`), 0o600)
	if s, err := LoadSites(obj); err != nil || len(s) != 1 || s[0].Name != "A" {
		t.Errorf("object form: %v %v", s, err)
	}
	if s, err := LoadSites(arr); err != nil || len(s) != 1 || s[0].Name != "B" {
		t.Errorf("array form: %v %v", s, err)
	}
	if _, err := LoadSites(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file should error")
	}
}

func TestDisabledRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "disabled.json")
	if m := LoadDisabled(p); len(m) != 0 {
		t.Errorf("missing file: %v", m)
	}
	if err := SaveDisabled(p, []string{"Foo", "Bar"}); err != nil {
		t.Fatal(err)
	}
	m := LoadDisabled(p)
	if !m["Foo"] || !m["Bar"] || len(m) != 2 {
		t.Errorf("round trip: %v", m)
	}
}

func TestRandomCanary(t *testing.T) {
	a, b := randomCanary(), randomCanary()
	if a == b || len(a) < 8 {
		t.Errorf("canaries must be unique and long: %q %q", a, b)
	}
}

func TestEmbeddedSitesAreWellFormed(t *testing.T) {
	sites, err := LoadSites("")
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, s := range sites {
		u := strings.ReplaceAll(s.URICheck, "{account}", "someone")
		pu, err := url.Parse(u)
		if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") || pu.Hostname() == "" || strings.ContainsAny(u, " \t\n") {
			t.Errorf("%s: bad URL %q", s.Name, s.URICheck)
			bad++
		}
		if s.Regex != "" && s.re == nil {
			t.Errorf("%s: username pattern %q did not compile", s.Name, s.Regex)
		}
		hasSignal := s.MString != "" || len(s.MStrings) > 0 || s.EString != "" || len(s.EStrings) > 0 || s.MCode != 0 || s.Check == checkRedirect || s.ECode != 0
		if !hasSignal {
			t.Errorf("%s: no signature at all", s.Name)
		}
		if s.PostBody != "" && !strings.Contains(s.PostBody, "{account}") && !strings.Contains(s.URICheck, "{account}") {
			t.Errorf("%s: POST body without a username placeholder", s.Name)
		}
		if bad > 20 {
			t.Fatal("too many malformed sites")
		}
	}
}

func TestScanKeepsTheFullTableOfOutcomes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/found/"):
			_, _ = w.Write([]byte("profile-of-user"))
		case strings.HasPrefix(r.URL.Path, "/absent/"):
			w.WriteHeader(404)
		case strings.HasPrefix(r.URL.Path, "/limited/"):
			w.WriteHeader(429)
		default:
			w.WriteHeader(418) // matches no signature and is not a success code
		}
	}))
	defer srv.Close()
	list := `[{"name":"Found","uri_check":"` + srv.URL + `/found/{account}","e_code":200,"e_string":"profile-of","m_code":404},
	 {"name":"Absent","uri_check":"` + srv.URL + `/absent/{account}","e_code":200,"m_code":404},
	 {"name":"Limited","uri_check":"` + srv.URL + `/limited/{account}","e_code":200,"m_code":404},
	 {"name":"Odd","uri_check":"` + srv.URL + `/odd/{account}","e_code":200,"m_code":404}]`
	path := filepath.Join(t.TempDir(), "sites.json")
	_ = os.WriteFile(path, []byte(list), 0o600)
	sm, err := New(Config{SitesFile: path})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := httpx.New(httpx.Options{Retries: 0})
	p := pool.New(4, 4)
	defer p.Close()
	var got []models.Finding
	var mu sync.Mutex
	err = sm.Scan(context.Background(), models.Target{Username: "bob"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { mu.Lock(); got = append(got, f); mu.Unlock() })
	if err != nil {
		t.Fatal(err)
	}
	var table string
	for _, f := range got {
		if f.Category == "toolrun" && f.Source == "social: every check" {
			table = f.Details["output"]
		}
	}
	for _, want := range []string{"Found\tbob\tfound\t200", "Absent\tbob\tnot found\t404", "Limited\tbob\tblocked or rate-limited\t429", "Odd\tbob\tinconclusive: the response matched no signature\t418"} {
		if !strings.Contains(table, want) {
			t.Errorf("table lacks %q:\n%s", want, table)
		}
	}
}
