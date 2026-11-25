package syswrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// extraTools are optional wrappers for heavier programs: SpiderFoot (an
// all-in-one OSINT framework) and h8mail (breach lookup). They run only when
// the program is installed, like every other wrapper.
func extraTools() []*Tool {
	return []*Tool{
		{
			// SpiderFoot's CLI prints its events as JSON. "-u passive" keeps it to
			// sources that never touch the target itself.
			Name: "spiderfoot", Binary: "spiderfoot", Timeout: 25 * time.Minute,
			Args: func(in Inputs) [][]string {
				var a [][]string
				for _, t := range []string{in.Domain, in.Email, in.Username, in.Phone} {
					if t != "" {
						a = append(a, []string{"-s", t, "-u", "passive", "-q", "-o", "json"})
					}
				}
				return a
			},
			Parse: parseSpiderFoot,
		},
		{
			// h8mail: only the breach/source names and counts are kept. The
			// credential values it can print are never stored or shown.
			Name: "h8mail", Binary: "h8mail", Requires: []string{"email"}, Timeout: 5 * time.Minute,
			ArgsDir: func(in Inputs, dir string) [][]string {
				return [][]string{{"-t", in.Email, "-j", filepath.Join(dir, "h8mail.json")}}
			},
			ParseDir: parseH8mail,
		},
	}
}

// parseSpiderFoot groups SpiderFoot's JSON events by type; every event's data
// is kept (nothing is dropped), one finding per event type.
func parseSpiderFoot(in Inputs, args []string, out string) []models.Finding {
	var events []struct {
		Type, Data, Module, Source string
		Generated                  json.Number `json:"generated"`
	}
	raw := strings.TrimSpace(out)
	if i := strings.Index(raw, "["); i > 0 {
		raw = raw[i:] // skip any banner printed before the JSON array
	}
	if json.Unmarshal([]byte(raw), &events) != nil {
		return nil
	}
	target := ""
	if len(args) > 1 {
		target = args[1]
	}
	groups := map[string][]string{}
	mods := map[string]map[string]bool{}
	for _, e := range events {
		t := strings.TrimSpace(e.Type)
		d := strings.TrimSpace(e.Data)
		if t == "" || d == "" || t == "ROOT" {
			continue
		}
		groups[t] = append(groups[t], d)
		if mods[t] == nil {
			mods[t] = map[string]bool{}
		}
		if e.Module != "" {
			mods[t][strings.TrimPrefix(e.Module, "sfp_")] = true
		}
	}
	types := make([]string, 0, len(groups))
	for t := range groups {
		types = append(types, t)
	}
	sort.Strings(types)
	var res []models.Finding
	for _, t := range types {
		items := dedupe(groups[t])
		var m []string
		for k := range mods[t] {
			m = append(m, k)
		}
		sort.Strings(m)
		d := map[string]string{"count": strconv.Itoa(len(items)), "values": strings.Join(items, "\n"), "spiderfoot modules": strings.Join(m, ", "), "target": target}
		res = append(res, finding("spiderfoot "+target+": "+t, "osint-framework", "", 0.6, d))
	}
	return res
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// parseH8mail reads h8mail's JSON report. Its "data" rows are string lists whose
// first element names the source (HIBP, a leak-lookup service…); later elements
// may be credentials. Only the source names and the number of rows are reported.
func parseH8mail(in Inputs, dir, _ string) []models.Finding {
	b, err := os.ReadFile(filepath.Join(dir, "h8mail.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		Targets []struct {
			Target string  `json:"target"`
			PwnNum float64 `json:"pwn_num"`
			Data   [][]any `json:"data"`
		} `json:"targets"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	var res []models.Finding
	for _, t := range doc.Targets {
		srcSet := map[string]bool{}
		for _, row := range t.Data {
			if len(row) == 0 {
				continue
			}
			if s, ok := row[0].(string); ok && s != "" && !strings.ContainsAny(s, ":@ ") {
				srcSet[s] = true
			}
		}
		if t.PwnNum == 0 && len(srcSet) == 0 && len(t.Data) == 0 {
			continue
		}
		var srcs []string
		for s := range srcSet {
			srcs = append(srcs, s)
		}
		sort.Strings(srcs)
		n := int(t.PwnNum)
		if n == 0 {
			n = len(t.Data)
		}
		d := map[string]string{"records found": strconv.Itoa(n), "sources": strings.Join(srcs, ", "),
			"note": "credential values are not stored or shown; only that the address appears in leaked data"}
		res = append(res, finding("h8mail: "+t.Target+" appears in leaked data", "breach", "", 0.7, d))
	}
	return res
}
