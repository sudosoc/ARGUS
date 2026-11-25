package social

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

//go:embed sites_default.json.gz
var defaultSitesGZ []byte

const accountToken = "{account}"

// Check modes.
const (
	checkSignature = ""         // status code + body markers (WMN style)
	checkRedirect  = "redirect" // exists iff the request answers 2xx without redirecting
)

// Tiers order sites by how much they are worth probing by default.
const (
	TierCore     = 1 // curated, popular, well-maintained signatures
	TierExtended = 2 // the long tail of ordinary platforms
	TierAll      = 3 // template-based forums and tiny communities
)

// Site describes how to test for a username on one platform. The base schema
// is the WhatsMyName (WMN) community format; the extra fields carry what the
// Maigret and Sherlock lists add (several markers per outcome, username
// patterns, redirect checks, popularity rank and a tier). cmd/sitesync builds
// the embedded list from all three sources.
type Site struct {
	Name       string            `json:"name"`
	URICheck   string            `json:"uri_check"`            // URL probed; contains {account}
	URIPretty  string            `json:"uri_pretty,omitempty"` // human-facing profile URL
	ECode      int               `json:"e_code,omitempty"`     // status when the account EXISTS (default 200)
	EString    string            `json:"e_string,omitempty"`   // body marker when it exists
	EStrings   []string          `json:"e_strings,omitempty"`  // any of these in the body means it exists
	MCode      int               `json:"m_code,omitempty"`     // status when it is MISSING
	MString    string            `json:"m_string,omitempty"`   // body marker when it is missing
	MStrings   []string          `json:"m_strings,omitempty"`  // any of these in the body means it is missing
	Errors     []string          `json:"errors,omitempty"`     // body markers meaning blocked / captcha / rate-limited
	Cat        string            `json:"cat,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Known      []string          `json:"known,omitempty"` // accounts known to exist (used by self-test)
	Headers    map[string]string `json:"headers,omitempty"`
	PostBody   string            `json:"post_body,omitempty"`
	Regex      string            `json:"regex,omitempty"`     // a username must match this to be probed
	Check      string            `json:"check,omitempty"`     // "" signatures, "redirect"
	Ignore403  bool              `json:"ignore403,omitempty"` // judge a 403 page by its content like any other
	Strip      string            `json:"strip_bad_char,omitempty"`
	Rank       int               `json:"rank,omitempty"`       // popularity rank, 1 = most visited, 0 = unknown
	Tier       int               `json:"tier,omitempty"`       // 1 core, 2 extended, 3 all (0 = core)
	Source     string            `json:"source,omitempty"`     // wmn, sherlock, maigret, custom
	Valid      *bool             `json:"valid,omitempty"`      // false = known broken, never probed
	Protection []string          `json:"protection,omitempty"` // anti-bot systems in front of the site

	subdomain bool
	re        *regexp.Regexp
}

var subdomainSafe = regexp.MustCompile(`^[A-Za-z0-9-]{1,63}$`)

// applicable reports whether user can legally appear in this site's URL
// (e.g. a "john.doe" cannot be a subdomain).
func (s *Site) applicable(user string) bool {
	user = s.clean(user)
	if user == "" || (s.re != nil && !s.re.MatchString(user)) {
		return false
	}
	if s.subdomain {
		return subdomainSafe.MatchString(user)
	}
	return true
}

// compile prepares the username pattern; an invalid pattern is dropped, not fatal.
func (s *Site) compile() error {
	if s.Regex == "" {
		return nil
	}
	re, err := regexp.Compile(s.Regex)
	if err != nil {
		s.Regex = ""
		return err
	}
	s.re = re
	return nil
}

// clean removes the characters the site cannot hold in a username.
func (s *Site) clean(user string) string {
	if s.Strip == "" {
		return user
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(s.Strip, r) {
			return -1
		}
		return r
	}, user)
}

// tier returns the effective tier (0 means core).
func (s *Site) tier() int {
	if s.Tier <= 0 {
		return TierCore
	}
	return s.Tier
}

func (s *Site) checkURL(user string) string {
	return strings.ReplaceAll(s.URICheck, accountToken, user)
}

func (s *Site) prettyURL(user string) string {
	u := s.URIPretty
	if u == "" {
		u = s.URICheck
	}
	return strings.ReplaceAll(u, accountToken, user)
}

// LoadSites parses sites from path, or the embedded default list when path is
// empty. Both a bare JSON array and the WMN object form {"sites":[...]} work.
func LoadSites(path string) ([]Site, error) {
	var data []byte
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		zr, err := gzip.NewReader(bytes.NewReader(defaultSitesGZ))
		if err != nil {
			return nil, fmt.Errorf("embedded site list: %w", err)
		}
		if data, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("embedded site list: %w", err)
		}
	}
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b { // a gzipped custom list
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if data, err = io.ReadAll(zr); err != nil {
			return nil, err
		}
	}

	var sites []Site
	var wrapped struct {
		Sites []Site `json:"sites"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Sites) > 0 {
		sites = wrapped.Sites
	} else if err := json.Unmarshal(data, &sites); err != nil {
		return nil, fmt.Errorf("parse sites: %w", err)
	}

	seen := make(map[string]bool, len(sites))
	out := sites[:0]
	for _, s := range sites {
		if s.Name == "" || !strings.Contains(s.URICheck+s.PostBody, accountToken) || seen[s.Name+s.URICheck] || (s.Valid != nil && !*s.Valid) {
			continue
		}
		_ = s.compile()
		seen[s.Name+s.URICheck] = true
		if s.ECode == 0 {
			s.ECode = 200
		}
		if i := strings.Index(s.URICheck, "://"); i >= 0 {
			host, _, _ := strings.Cut(s.URICheck[i+3:], "/")
			s.subdomain = strings.Contains(host, accountToken)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no usable sites found")
	}
	return out, nil
}
