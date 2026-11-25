package sitesync

import (
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/modules/social"
)

const wmnJSON = `{"sites":[
 {"name":"GitHub","uri_check":"https://github.com/{account}","e_code":200,"e_string":"vcard","m_code":404,"m_string":"Not Found","cat":"coding","known":["torvalds"]},
 {"name":"Broken","uri_check":"https://b.example/{account}","e_code":200,"m_code":404,"cat":"misc","valid":false},
 {"name":"Walled","uri_check":"https://w.example/{account}","e_code":200,"m_code":404,"cat":"misc","protection":["anubis"]}]}`

const sherlockJSON = `{"$schema":"x",
 "GitHub":{"errorType":"status_code","url":"https://www.github.com/{}","urlMain":"https://github.com","username_claimed":"blue"},
 "Msg":{"errorType":"message","errorMsg":["gone","missing"],"url":"https://msg.example/u/{}","regexCheck":"^[a-z]+$","username_claimed":"a"},
 "Redir":{"errorType":"response_url","errorUrl":"https://r.example/err","url":"https://r.example/{}","username_claimed":"a"}}`

const maigretJSON = `{"engines":{"XenForo":{"name":"XenForo","site":{"checkType":"message","url":"{urlMain}{urlSubpath}/members/?username={username}","absenceStrs":["not found"],"presenseStrs":["memberHeader"]}}},
 "tags":[],
 "sites":{
 "Popular":{"url":"https://popular.example/{username}","urlMain":"https://popular.example","checkType":"status_code","alexaRank":1200,"tags":["music","us"],"usernameClaimed":"x"},
 "Forum":{"engine":"XenForo","urlMain":"https://forum.example","urlSubpath":"/f","usernameClaimed":"admin","tags":["forum"]},
 "Api":{"url":"https://api.example/@{username}","urlProbe":"https://api.example/v1/{username}","urlMain":"https://api.example","checkType":"message","presenseStrs":["\"id\":"],"absenceStrs":["null"],"errors":{"captcha":"c"},"ignore403":true},
 "Off":{"url":"https://off.example/{username}","urlMain":"https://off.example","checkType":"status_code","disabled":true},
 "IDType":{"url":"https://id.example/{username}","urlMain":"https://id.example","type":"gaia_id","checkType":"status_code"},
 "TLS":{"url":"https://tls.example/{username}","urlMain":"https://tls.example","checkType":"status_code","protection":["tls_fingerprint"]},
 "Adult":{"url":"https://adult.example/{username}","urlMain":"https://adult.example","checkType":"status_code","tags":["porn"]},
 "GitHubDup":{"url":"https://github.com/{username}","urlMain":"https://github.com","checkType":"status_code","alexaRank":80,"tags":["coding"],"usernameClaimed":"octocat"},
 "Post":{"url":"https://post.example/","urlMain":"https://post.example","requestMethod":"POST","requestPayload":{"username":"{username}"},"checkType":"message","absenceStrs":["no"]}
 }}`

func byName(sites []social.Site, n string) *social.Site {
	for i := range sites {
		if sites[i].Name == n {
			return &sites[i]
		}
	}
	return nil
}

func TestConvertWMN(t *testing.T) {
	s, err := ConvertWMN([]byte(wmnJSON))
	if err != nil || len(s) != 1 || s[0].Name != "GitHub" || s[0].Source != SrcWMN || s[0].Tier != social.TierCore {
		t.Fatalf("%v %+v", err, s)
	}
	if s[0].MString != "Not Found" || s[0].EString != "vcard" || s[0].Known[0] != "torvalds" {
		t.Errorf("fields lost: %+v", s[0])
	}
}

func TestConvertSherlock(t *testing.T) {
	s, err := ConvertSherlock([]byte(sherlockJSON))
	if err != nil || len(s) != 3 {
		t.Fatalf("%v %d", err, len(s))
	}
	if g := byName(s, "GitHub"); g == nil || g.URICheck != "https://www.github.com/{account}" || g.MCode != 404 {
		t.Errorf("status_code site: %+v", g)
	}
	if m := byName(s, "Msg"); m == nil || len(m.MStrings) != 2 || m.Regex != "^[a-z]+$" || m.ECode != 200 {
		t.Errorf("message site: %+v", m)
	}
	if r := byName(s, "Redir"); r == nil || r.Check != "redirect" {
		t.Errorf("redirect site: %+v", r)
	}
}

func TestConvertMaigret(t *testing.T) {
	s, err := ConvertMaigret([]byte(maigretJSON))
	if err != nil {
		t.Fatal(err)
	}
	for _, skip := range []string{"Off", "IDType", "TLS"} {
		if byName(s, skip) != nil {
			t.Errorf("%s must be skipped", skip)
		}
	}
	p := byName(s, "Popular")
	if p == nil || p.Tier != social.TierCore || p.Rank != 1200 || p.Cat != "music" || p.MCode != 404 {
		t.Errorf("popular: %+v", p)
	}
	f := byName(s, "Forum")
	if f == nil || f.Tier != social.TierAll || f.URICheck != "https://forum.example/f/members/?username={account}" || len(f.EStrings) == 0 || f.MStrings[0] != "not found" {
		t.Errorf("engine expansion: %+v", f)
	}
	a := byName(s, "Api")
	if a == nil || a.URICheck != "https://api.example/v1/{account}" || a.URIPretty != "https://api.example/@{account}" || !a.Ignore403 || a.Errors[0] != "captcha" {
		t.Errorf("api site: %+v", a)
	}
	if ad := byName(s, "Adult"); ad == nil || !strings.Contains(ad.Cat, "NSFW") {
		t.Errorf("adult site must be categorised NSFW: %+v", ad)
	}
	po := byName(s, "Post")
	if po == nil || po.PostBody != `{"username":"{account}"}` || po.Headers["Content-Type"] != "application/json" {
		t.Errorf("post site: %+v", po)
	}
}

func TestMergePrioritiesAndDedupe(t *testing.T) {
	w, _ := ConvertWMN([]byte(wmnJSON))
	sh, _ := ConvertSherlock([]byte(sherlockJSON))
	mg, _ := ConvertMaigret([]byte(maigretJSON))
	out := Merge(w, sh, mg)
	gh := byName(out, "GitHub")
	if gh == nil || gh.Source != SrcWMN {
		t.Fatalf("WMN must win the shared github.com check: %+v", gh)
	}
	if gh.Rank != 80 {
		t.Errorf("rank must be filled in from Maigret: %d", gh.Rank)
	}
	known := strings.Join(gh.Known, ",")
	if !strings.Contains(known, "torvalds") || !strings.Contains(known, "octocat") {
		t.Errorf("known accounts must be merged: %v", gh.Known)
	}
	names := map[string]bool{}
	for _, s := range out {
		if names[strings.ToLower(s.Name)] {
			t.Errorf("duplicate name %q", s.Name)
		}
		names[strings.ToLower(s.Name)] = true
	}
	for i := 1; i < len(out); i++ {
		if out[i-1].Tier > out[i].Tier {
			t.Fatal("output must be ordered by tier")
		}
	}
}

func TestKey(t *testing.T) {
	a := Key(social.Site{URICheck: "https://www.Example.com/u/{account}/"})
	b := Key(social.Site{URICheck: "http://example.com/u/{account}"})
	if a != b || a != "example.com/u/{account}" {
		t.Errorf("%q %q", a, b)
	}
}
