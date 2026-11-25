// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// associates ranks who and what is close to the subject: accounts they follow and are followed by, the
// organisations and schools their profiles state. Strength grows with the number of independent sources
// and with reciprocity. A public connection is a lead about interaction, not proof of a relationship.
func associates(c *ctx) []models.Associate {
	agg := map[string]*models.Associate{}
	put := func(kind, name string, strength float64, why, src, url string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		k := kind + "|" + strings.ToLower(name)
		if e, ok := agg[k]; ok {
			e.Strength = 1 - (1-e.Strength)*(1-strength) // independent evidence combines
			if !containsFold(e.Sources, src) {
				e.Sources = append(e.Sources, src)
			}
			if !strings.Contains(e.Why, why) {
				e.Why += "; " + why
			}
			return
		}
		agg[k] = &models.Associate{Kind: kind, Name: name, Strength: strength, Why: why, Sources: []string{src}, URL: url}
	}

	for i := range c.hits {
		f := c.hits[i]
		switch f.Module {
		case "bskygraph":
			for _, m := range strings.Split(f.Details["mutual follows"], ";") {
				put("person", m, 0.8, "follows the subject and is followed back (Bluesky)", "Bluesky follow lists", f.URL)
			}
			for _, line := range strings.Split(f.Details["likely connections"], "\n") {
				name, rest, ok := strings.Cut(line, " — followed by ")
				if !ok {
					continue
				}
				// "N of the M accounts checked"
				var n, m int
				fmt.Sscanf(rest, "%d of the %d", &n, &m)
				s := 0.3
				if m > 0 {
					s = 0.3 + 0.5*float64(n)/float64(m)
				}
				put("person", name, s, "followed by "+strconv.Itoa(n)+" of the subject's follows (Bluesky)", "Bluesky follow lists", f.URL)
			}
		case "devtrace":
			for _, o := range strings.Split(f.Details["public organisations"], ",") {
				put("organisation", o, 0.6, "public GitHub organisation", "GitHub", f.URL)
			}
		}
	}
	if c.primary != nil {
		for _, e := range c.primary.Employers {
			s := 0.5 + 0.12*float64(len(e.Sources))
			if e.Confirmed {
				s += 0.15
			}
			put("organisation", e.Value, minf(s, 0.95), "employer stated by "+strings.Join(e.Sources, ", "), "profiles", "")
		}
		for _, e := range c.primary.Education {
			s := 0.45 + 0.12*float64(len(e.Sources))
			put("school", e.Value, minf(s, 0.9), "education stated by "+strings.Join(e.Sources, ", "), "profiles", "")
		}
	}
	out := make([]models.Associate, 0, len(agg))
	for _, v := range agg {
		v.Strength = float64(int(v.Strength*100)) / 100
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
