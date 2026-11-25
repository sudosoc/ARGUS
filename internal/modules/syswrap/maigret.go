package syswrap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/util"
)

// maigretLine is one claimed account in Maigret's ndjson report. Maigret
// serialises a lot more; only the parts that identify a person are read.
type maigretLine struct {
	SiteName string `json:"sitename"`
	URLUser  string `json:"url_user"`
	Status   struct {
		SiteName string         `json:"site_name"`
		URL      string         `json:"url"`
		IDs      map[string]any `json:"ids"`
		Tags     []string       `json:"tags"`
	} `json:"status"`
	IDsUsernames map[string]any `json:"ids_usernames"`
	IDsLinks     []any          `json:"ids_links"`
	Rank         int            `json:"rank"`
	HTTPStatus   any            `json:"http_status"`
}

// parseMaigretDir reads the report Maigret wrote into dir. Every claimed
// account becomes a finding carrying the profile data Maigret extracted from
// the page, and the usernames, links and images it found become entities the
// engine can follow.
func parseMaigretDir(in Inputs, dir, _ string) []models.Finding {
	files, _ := filepath.Glob(filepath.Join(dir, "report_*_ndjson.json"))
	var res []models.Finding
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
		for sc.Scan() && len(res) < maxItems*2 {
			var l maigretLine
			if json.Unmarshal(sc.Bytes(), &l) != nil {
				continue
			}
			if fnd, ok := maigretFinding(in, l); ok {
				res = append(res, fnd)
			}
		}
		f.Close()
	}
	return res
}

func maigretFinding(in Inputs, l maigretLine) (models.Finding, bool) {
	site := util.FirstNonEmpty(l.SiteName, l.Status.SiteName)
	link := util.FirstNonEmpty(l.URLUser, l.Status.URL)
	if site == "" || link == "" {
		return models.Finding{}, false
	}
	d := map[string]string{"username": in.Username, "via": "maigret"}
	if len(l.Status.Tags) > 0 {
		d["tags"] = strings.Join(util.ClipList(l.Status.Tags, 8), ", ")
	}
	if l.Rank > 0 {
		d["site rank"] = fmt.Sprint(l.Rank)
	}
	conf := 0.7
	var ents []models.Entity
	keys := make([]string, 0, len(l.Status.IDs))
	for k := range l.Status.IDs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := strings.TrimSpace(fmt.Sprint(l.Status.IDs[k]))
		if v == "" || v == "<nil>" || v == "[]" {
			continue
		}
		switch k {
		case "fullname", "name", "display_name":
			d["name"] = util.Clip(v, 120)
			ents = append(ents, models.Entity{Type: models.EntName, Value: util.Clip(v, 120)})
		case "image", "avatar", "avatar_url", "profile_image":
			if u, err := url.Parse(v); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
				d["avatar"] = util.Clip(v, 300)
			}
		case "bio", "about", "description":
			d["bio"] = util.Clip(v, 300)
		case "location", "country", "city":
			d[k] = util.Clip(v, 100)
			ents = append(ents, models.Entity{Type: models.EntLocation, Value: util.Clip(v, 100)})
		case "email":
			d["email"] = util.Clip(v, 120)
			ents = append(ents, models.Entity{Type: models.EntEmail, Value: strings.ToLower(util.Clip(v, 120))})
		case "website", "links", "created_at", "follower_count", "following_count", "gender", "birthday", "company", "uid", "id":
			d[k] = util.Clip(v, 200)
		}
	}
	if len(d) > 4 { // the page yielded real profile data beyond "exists"
		conf = 0.8
	}
	for name := range l.IDsUsernames {
		if n := strings.TrimSpace(name); n != "" && !strings.EqualFold(n, in.Username) {
			ents = append(ents, models.Entity{Type: models.EntUsername, Value: n})
		}
	}
	fnd := models.Finding{Source: site, Category: "social", URL: link, Status: models.StatusFound, Confidence: conf, Details: d, Entities: ents}
	return fnd, true
}
