// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// explain says, for every fact about the best-matching person, why it is trusted as much as it is.
func explain(c *ctx) []models.Explanation {
	if c.primary == nil {
		return nil
	}
	P := c.primary
	groups := []struct {
		attr  string
		facts []models.Fact
	}{
		{"name", P.Names}, {"headline", P.Headlines}, {"employer", P.Employers}, {"location", P.Locations}, {"education", P.Education},
		{"e-mail", P.Emails}, {"phone", P.Phones}, {"username", P.Usernames}, {"website", P.Websites}, {"interest", P.Interests},
	}
	firstSeen := func(sources []string) string {
		best := ""
		for _, ev := range c.rep.Timeline {
			for _, s := range sources {
				if s != "" && strings.Contains(strings.ToLower(ev.Source), strings.ToLower(s)) && isCreation(ev.Event) {
					if best == "" || ev.Date < best {
						best = ev.Date
					}
				}
			}
		}
		if len(best) >= 10 {
			return best[:10]
		}
		return ""
	}
	var out []models.Explanation
	for _, g := range groups {
		for _, f := range g.facts {
			e := models.Explanation{Attribute: g.attr, Value: f.Value, Sources: f.Sources, Earliest: firstSeen(f.Sources)}
			for _, conf := range P.Conflicts {
				if strings.Contains(strings.ToLower(conf), g.attr) || (g.attr == "location" && strings.Contains(strings.ToLower(conf), "place")) {
					e.Against = append(e.Against, conf)
				}
			}
			n := len(f.Sources)
			switch {
			case len(e.Against) > 0 && g.attr != "interest":
				e.Level = "contested"
				e.Why = fmt.Sprintf("Stated by %s, but the sources disagree about this attribute: %s.", joinSources(f.Sources), strings.Join(e.Against, "; "))
			case f.Confirmed:
				e.Level = "confirmed"
				e.Why = fmt.Sprintf("Stated by %d independent platforms (%s), so one wrong profile cannot explain it.", n, joinSources(f.Sources))
			case f.Self:
				e.Level = "self-reported"
				e.Why = fmt.Sprintf("Stated only in the person's own profile (%s). It is what they say about themself and has not been checked against a second source.", joinSources(f.Sources))
			default:
				e.Level = "single source"
				e.Why = fmt.Sprintf("Stated by one third-party source only (%s). Treat it as a lead.", joinSources(f.Sources))
			}
			if f.Trust > 0 {
				e.Why += fmt.Sprintf(" Source reliability %d%%.", int(f.Trust*100))
			}
			out = append(out, e)
		}
	}
	rank := map[string]int{"confirmed": 0, "contested": 1, "self-reported": 2, "single source": 3}
	// stable sort by level
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && rank[out[j].Level] < rank[out[j-1].Level]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func joinSources(s []string) string {
	if len(s) == 0 {
		return "an unnamed source"
	}
	return strings.Join(s, ", ")
}
