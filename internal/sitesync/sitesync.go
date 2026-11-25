// Package sitesync turns the community username-site lists (WhatsMyName,
// Sherlock, Maigret) into one de-duplicated list in ARGUS's own schema, and
// assigns each site a tier so scans can start with the best-maintained
// platforms and go deeper only when asked. It is used by cmd/sitesync; the
// result is embedded into the binary.
package sitesync

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/sudosoc/argus/internal/modules/social"
)

// Source names stored in Site.Source.
const (
	SrcWMN      = "wmn"
	SrcSherlock = "sherlock"
	SrcMaigret  = "maigret"
)

// coreRank is the popularity rank (lower = more visited) up to which a
// Maigret site is considered core rather than extended.
const coreRank = 300000

// blockedProtections are anti-bot systems a plain HTTP client cannot pass.
// ARGUS does not evade protections, so sites behind them are left out.
var blockedProtections = map[string]bool{
	"tls_fingerprint": true, "cf_js_challenge": true, "js_challenge": true, "aws_waf_js_challenge": true,
	"ddos_guard_challenge": true, "login": true, "custom_bot_protection": true, "user-auth": true, "anubis": true,
}

var nsfwTags = map[string]bool{"porn": true, "erotic": true, "webcam": true, "adult": true, "nsfw": true}

// tagToCat maps Maigret tags onto the WhatsMyName category names the UI and
// the -categories filter already know.
var tagToCat = map[string]string{
	"art": "art", "design": "art", "3d": "art", "blog": "blog", "writing": "blog", "business": "business", "career": "business",
	"professional": "business", "freelance": "business", "coding": "coding", "hacking": "coding", "dating": "dating",
	"finance": "finance", "fintech": "finance", "crypto": "finance", "stock": "finance", "trading": "finance", "nft": "finance",
	"gaming": "gaming", "hobby": "hobby", "cooking": "hobby", "fashion": "hobby", "travel": "hobby", "photo": "images",
	"music": "music", "news": "news", "military": "political", "religion": "political", "shopping": "shopping",
	"classified": "shopping", "social": "social", "messaging": "social", "networking": "social", "geosocial": "social",
	"tech": "tech", "apps": "tech", "science": "tech", "research": "tech", "education": "tech", "streaming": "video",
	"video": "video", "movies": "video", "anime": "video", "medicine": "health", "psychology": "health", "archive": "archived",
	"forum": "social", "discussion": "social",
}

var dblSlash = regexp.MustCompile(`([^:])//+`)

func str(v any) string { s, _ := v.(string); return s }

func strList(v any) []string {
	switch t := v.(type) {
	case string:
		if t != "" {
			return []string{t}
		}
	case []any:
		var out []string
		for _, x := range t {
			if s := str(x); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func strMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, x := range m {
		out[k] = str(x)
	}
	return out
}

// payload renders a request body from a JSON object (Maigret/Sherlock
// payloads are objects with a username placeholder) or a plain string.
func payload(v any, placeholder string) (body string, json bool) {
	switch t := v.(type) {
	case string:
		return strings.ReplaceAll(t, placeholder, "{account}"), false
	case map[string]any:
		b, err := jsonMarshal(t)
		if err != nil {
			return "", false
		}
		return strings.ReplaceAll(string(b), placeholder, "{account}"), true
	}
	return "", false
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// ConvertWMN reads the WhatsMyName wmn-data.json format.
func ConvertWMN(data []byte) ([]social.Site, error) {
	var w struct {
		Sites []map[string]any `json:"sites"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("wmn: %w", err)
	}
	var out []social.Site
	for _, m := range w.Sites {
		if v, ok := m["valid"].(bool); ok && !v {
			continue
		}
		prot := strList(m["protection"])
		if hasAny(prot, blockedProtections) {
			continue
		}
		s := social.Site{
			Name: str(m["name"]), URICheck: str(m["uri_check"]), URIPretty: str(m["uri_pretty"]),
			EString: str(m["e_string"]), MString: str(m["m_string"]), Cat: str(m["cat"]),
			Known: strList(m["known"]), Headers: strMap(m["headers"]), Strip: str(m["strip_bad_char"]),
			Protection: prot, Source: SrcWMN, Tier: social.TierCore,
		}
		s.ECode, s.MCode = int(num(m["e_code"])), int(num(m["m_code"]))
		s.PostBody = str(m["post_body"])
		out = append(out, s)
	}
	return out, nil
}

func num(v any) float64 { f, _ := v.(float64); return f }

func hasAny(list []string, set map[string]bool) bool {
	for _, x := range list {
		if set[x] {
			return true
		}
	}
	return false
}

// ConvertSherlock reads Sherlock's resources/data.json.
func ConvertSherlock(data []byte) ([]social.Site, error) {
	var raw map[string]map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		// "$schema" is a string, not an object; decode loosely.
		var loose map[string]json.RawMessage
		if err2 := json.Unmarshal(data, &loose); err2 != nil {
			return nil, fmt.Errorf("sherlock: %w", err)
		}
		raw = map[string]map[string]any{}
		for k, v := range loose {
			var m map[string]any
			if json.Unmarshal(v, &m) == nil {
				raw[k] = m
			}
		}
	}
	names := make([]string, 0, len(raw))
	for n := range raw {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []social.Site
	for _, name := range names {
		m := raw[name]
		if m["url"] == nil {
			continue
		}
		u := strings.ReplaceAll(str(m["url"]), "{}", "{account}")
		s := social.Site{Name: name, URIPretty: u, URICheck: u, Cat: "misc", Source: SrcSherlock, Tier: social.TierCore,
			Headers: strMap(m["headers"]), Regex: str(m["regexCheck"])}
		if p := str(m["urlProbe"]); p != "" {
			s.URICheck = strings.ReplaceAll(p, "{}", "{account}")
		}
		if c := str(m["username_claimed"]); c != "" {
			s.Known = []string{c}
		}
		if b, ok := m["isNSFW"].(bool); ok && b {
			s.Cat = "xx NSFW xx"
		}
		switch str(m["errorType"]) {
		case "message":
			s.ECode = 200
			s.MStrings = strList(m["errorMsg"])
		case "response_url":
			s.Check = "redirect"
		default: // status_code
			s.ECode, s.MCode = 200, 404
		}
		if body, _ := payload(m["request_payload"], "{}"); body != "" {
			s.PostBody = body
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			if _, ok := s.Headers["Content-Type"]; !ok {
				s.Headers["Content-Type"] = "application/json"
			}
		}
		if strings.EqualFold(str(m["request_method"]), "HEAD") || strings.EqualFold(str(m["request_method"]), "PUT") {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// ConvertMaigret reads Maigret's resources/data.json, expanding its engine
// templates (XenForo, phpBB, …) into ordinary sites.
func ConvertMaigret(data []byte) ([]social.Site, error) {
	var d struct {
		Sites   map[string]map[string]any `json:"sites"`
		Engines map[string]struct {
			Site map[string]any `json:"site"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("maigret: %w", err)
	}
	names := make([]string, 0, len(d.Sites))
	for n := range d.Sites {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []social.Site
	for _, name := range names {
		m := d.Sites[name]
		if b, _ := m["disabled"].(bool); b {
			continue
		}
		if t := str(m["type"]); t != "" && t != "username" {
			continue
		}
		if m["activation"] != nil || m["similarSearch"] != nil || m["encoding"] != nil || m["requestHeadOnly"] != nil {
			continue
		}
		if meth := strings.ToUpper(str(m["requestMethod"])); meth != "" && meth != "GET" && meth != "POST" {
			continue
		}
		prot := strList(m["protection"])
		if hasAny(prot, blockedProtections) {
			continue
		}
		merged := map[string]any{}
		engine := str(m["engine"])
		if e, ok := d.Engines[engine]; ok {
			for k, v := range e.Site {
				merged[k] = v
			}
		}
		for k, v := range m {
			merged[k] = v
		}
		urlMain, sub := str(merged["urlMain"]), str(merged["urlSubpath"])
		expand := func(t string) string {
			t = strings.NewReplacer("{urlMain}", urlMain, "{urlSubpath}", sub, "{username}", "{account}").Replace(t)
			return dblSlash.ReplaceAllString(t, "$1/")
		}
		pretty := expand(str(merged["url"]))
		check := pretty
		if p := str(merged["urlProbe"]); p != "" {
			check = expand(p)
		}
		body, isJSON := payload(merged["requestPayload"], "{username}")
		if !strings.Contains(check, "{account}") && !strings.Contains(body, "{account}") {
			continue
		}
		s := social.Site{
			Name: name, URICheck: check, URIPretty: pretty, Source: SrcMaigret, Protection: prot,
			Regex: str(merged["regexCheck"]), Ignore403: merged["ignore403"] == true,
			Headers: strMap(merged["headers"]), Tags: strList(merged["tags"]),
		}
		if c := str(merged["usernameClaimed"]); c != "" {
			s.Known = []string{c}
		}
		if r := num(merged["alexaRank"]); r > 0 {
			s.Rank = int(r)
		}
		for k := range strMap(merged["errors"]) {
			s.Errors = append(s.Errors, k)
		}
		sort.Strings(s.Errors)
		ct := str(merged["checkType"])
		abs, pre := strList(merged["absenceStrs"]), strList(merged["presenseStrs"])
		if ct == "" {
			if len(abs) > 0 || len(pre) > 0 {
				ct = "message"
			} else {
				ct = "status_code"
			}
		}
		switch ct {
		case "message":
			s.ECode, s.MStrings, s.EStrings = 200, abs, pre
		case "response_url":
			s.Check = "redirect"
		default:
			s.ECode, s.MCode = 200, 404
		}
		if body != "" {
			s.PostBody = body
			if s.Headers == nil {
				s.Headers = map[string]string{}
			}
			if isJSON {
				s.Headers["Content-Type"] = "application/json"
			}
		}
		s.Cat = "misc"
		for _, t := range s.Tags {
			if nsfwTags[t] {
				s.Cat = "xx NSFW xx"
				break
			}
		}
		if s.Cat == "misc" {
			for _, t := range s.Tags {
				if c, ok := tagToCat[t]; ok {
					s.Cat = c
					break
				}
			}
		}
		switch {
		case engine != "":
			s.Tier = social.TierAll
		case s.Rank > 0 && s.Rank <= coreRank:
			s.Tier = social.TierCore
		default:
			s.Tier = social.TierExtended
		}
		if s.Rank > 0 && s.Rank <= coreRank && engine != "" {
			s.Tier = social.TierExtended
		}
		out = append(out, s)
	}
	return out, nil
}

// Key identifies the same check across lists: host + path pattern, without
// scheme or a leading "www.".
func Key(s social.Site) string {
	k := strings.ToLower(strings.TrimSpace(s.URICheck))
	if i := strings.Index(k, "://"); i >= 0 {
		k = k[i+3:]
	}
	k = strings.TrimPrefix(k, "www.")
	k = strings.TrimSuffix(k, "/")
	return k
}

// Merge combines lists in priority order (earlier lists win the signature),
// merging known accounts, tags and the popularity rank from every source and
// keeping the lowest tier. Names are made unique.
func Merge(lists ...[]social.Site) []social.Site {
	byKey := map[string]int{}
	var out []social.Site
	for _, list := range lists {
		for _, s := range list {
			if s.Name == "" || !strings.Contains(s.URICheck+s.PostBody, "{account}") {
				continue
			}
			// The same platform shows up under its API URL in one list and its
			// profile URL in another: match on either.
			keys := []string{Key(s)}
			if s.URIPretty != "" && s.URIPretty != s.URICheck {
				keys = append(keys, Key(social.Site{URICheck: s.URIPretty}))
			}
			if s.PostBody != "" {
				keys = []string{keys[0] + "|" + s.PostBody}
			}
			i, seen := -1, false
			for _, k := range keys {
				if j, ok := byKey[k]; ok {
					i, seen = j, true
					break
				}
			}
			if !seen {
				for _, k := range keys {
					byKey[k] = len(out)
				}
				out = append(out, s)
				continue
			}
			for _, k := range keys {
				byKey[k] = i
			}
			w := &out[i]
			if s.Rank > 0 && (w.Rank == 0 || s.Rank < w.Rank) {
				w.Rank = s.Rank
			}
			if t := s.Tier; t > 0 && (w.Tier == 0 || t < w.Tier) {
				w.Tier = t
			}
			w.Known = union(w.Known, s.Known)
			w.Tags = union(w.Tags, s.Tags)
			if w.Cat == "misc" && s.Cat != "misc" && s.Cat != "" {
				w.Cat = s.Cat
			}
		}
	}
	// A popular site (any rank known) belongs in the core tier.
	for i := range out {
		if out[i].Rank > 0 && out[i].Rank <= coreRank && out[i].Tier > social.TierCore && out[i].Source != SrcMaigret {
			out[i].Tier = social.TierCore
		}
	}
	names := map[string]int{}
	for i := range out {
		names[strings.ToLower(out[i].Name)]++
	}
	seen := map[string]int{}
	for i := range out {
		l := strings.ToLower(out[i].Name)
		if names[l] > 1 {
			seen[l]++
			host := Key(out[i])
			if j := strings.IndexAny(host, "/?"); j > 0 {
				host = host[:j]
			}
			out[i].Name = fmt.Sprintf("%s (%s)", out[i].Name, host)
		}
	}
	used := map[string]int{}
	for i := range out {
		l := strings.ToLower(out[i].Name)
		if used[l]++; used[l] > 1 {
			out[i].Name = fmt.Sprintf("%s #%d", out[i].Name, used[l])
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func union(a, b []string) []string {
	have := map[string]bool{}
	for _, x := range a {
		have[x] = true
	}
	for _, x := range b {
		if !have[x] {
			a = append(a, x)
			have[x] = true
		}
	}
	return a
}

// Stats counts a merged list by tier and source.
func Stats(sites []social.Site) (byTier map[int]int, bySource map[string]int) {
	byTier, bySource = map[int]int{}, map[string]int{}
	for _, s := range sites {
		t := s.Tier
		if t == 0 {
			t = 1
		}
		byTier[t]++
		bySource[s.Source]++
	}
	return
}
