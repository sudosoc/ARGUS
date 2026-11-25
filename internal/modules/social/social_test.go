package social

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
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
	if len(sites) < 500 {
		t.Fatalf("embedded list suspiciously small: %d", len(sites))
	}
	seen := map[string]bool{}
	for _, s := range sites {
		if s.Name == "" || s.URICheck == "" {
			t.Fatalf("site without name or URL: %+v", s)
		}
		if !contains(s.URICheck, "{account}") {
			t.Errorf("%s: uri_check lacks {account}", s.Name)
		}
		if seen[s.Name] {
			t.Errorf("duplicate site name %q", s.Name)
		}
		seen[s.Name] = true
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
