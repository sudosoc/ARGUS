// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// independent are the modules whose records come from a party other than the person.
var independent = map[string]bool{"people": true, "records": true, "company": true, "bizreg": true, "sanctions": true}

// authenticity scores how much the identity behaves like an established real person. Every factor is listed
// with the points it added or removed, so the score can be argued with. It is a set of signals, not a verdict.
func authenticity(c *ctx) *models.Authenticity {
	if c.primary == nil {
		return nil
	}
	P := c.primary
	a := &models.Authenticity{}
	score := 50
	add := func(name string, eff int, detail string) {
		a.Factors = append(a.Factors, models.Factor{Name: name, Effect: eff, Detail: detail})
		score += eff
	}

	// age of the oldest account
	var first time.Time
	for _, ev := range c.rep.Timeline {
		if isCreation(ev.Event) {
			if t, ok := parseDate(ev.Date); ok && (first.IsZero() || t.Before(first)) {
				first = t
			}
		}
	}
	switch years := time.Since(first).Hours() / 24 / 365.25; {
	case first.IsZero():
		a.Factors = append(a.Factors, models.Factor{Name: "Age of accounts", Effect: 0, Detail: "no account creation date was found"})
	case years >= 8:
		add("Age of accounts", 15, fmt.Sprintf("the oldest dated account is %.0f years old", years))
	case years >= 3:
		add("Age of accounts", 8, fmt.Sprintf("the oldest dated account is %.0f years old", years))
	case years < 1:
		add("Age of accounts", -12, "every dated account is under a year old")
	default:
		add("Age of accounts", 2, fmt.Sprintf("the oldest dated account is %.1f years old", years))
	}

	// independent platforms agreeing
	plats := map[string]bool{}
	for _, r := range P.Records {
		plats[r.Platform] = true
	}
	switch n := len(plats); {
	case n >= 4:
		add("Independent sources", 10, fmt.Sprintf("%d different platforms describe this person", n))
	case n >= 2:
		add("Independent sources", 5, fmt.Sprintf("%d platforms describe this person", n))
	default:
		add("Independent sources", -10, "a single source describes this person")
	}

	// names
	confirmedName := false
	for _, n := range P.Names {
		if n.Confirmed {
			confirmedName = true
		}
	}
	if confirmedName {
		add("Name", 8, "the name is stated by at least two independent platforms")
	} else if len(P.Names) == 0 {
		add("Name", -6, "no profile states a name")
	}

	// third parties
	third := 0
	for _, f := range c.hits {
		if independent[f.Module] && f.Record != nil {
			third++
		}
	}
	if third > 0 {
		add("Third-party records", 10, fmt.Sprintf("%s (registers, research databases, encyclopaedias) mention the person", plural(third, "record", "records")))
	}
	selfRec := 0
	for _, r := range P.Records {
		if r.Self {
			selfRec++
		}
	}
	if selfRec > 0 {
		add("Self-authored pages", 5, fmt.Sprintf("%s written by the person (profile, CV, home page)", plural(selfRec, "page", "pages")))
	}

	// keys
	for _, f := range c.hits {
		if f.Module == "keys" {
			add("Published cryptographic key", 5, "an OpenPGP key is published for the person's address")
			break
		}
	}

	// copied bios
	for _, f := range c.hits {
		if f.Module == "integrity" {
			add("Copied profile text", -15, "the same bio appears under different names: one profile may be a copy")
			break
		}
	}

	// pictures
	a.Accounts = accountFlags(c)
	reusedWrong := 0
	for _, af := range a.Accounts {
		for _, fl := range af.Flags {
			if strings.HasPrefix(fl, "picture also used under a different name") {
				reusedWrong++
			}
		}
	}
	if reusedWrong > 0 {
		add("Picture reuse", -15, "a picture is used under a different name elsewhere")
	} else if len(P.Photos) > 0 {
		add("Picture", 3, "the person publishes a picture of their own")
	}

	// conflicts
	if n := len(P.Conflicts); n > 0 {
		add("Contradictions", -clamp(4*n, 4, 12), fmt.Sprintf("%s between sources (%s)", plural(n, "attribute disagrees", "attributes disagree"), strings.Join(P.Conflicts, ", ")))
	}

	// continuity
	years := map[int]bool{}
	for _, ev := range c.rep.Timeline {
		if t, ok := parseDate(ev.Date); ok {
			years[t.Year()] = true
		}
	}
	if len(years) >= 3 {
		add("Continuity", 6, fmt.Sprintf("dated activity in %d different years", len(years)))
	}

	a.Score = clamp(score, 0, 100)
	switch {
	case a.Score >= 75:
		a.Label = "Established"
	case a.Score >= 55:
		a.Label = "Plausible"
	case a.Score >= 35:
		a.Label = "Thin"
	default:
		a.Label = "Doubtful"
	}
	sort.SliceStable(a.Factors, func(i, j int) bool { return abs(a.Factors[i].Effect) > abs(a.Factors[j].Effect) })
	return a
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// accountFlags notes what is odd or reassuring about each of the person's accounts.
func accountFlags(c *ctx) []models.AccountFlag {
	if c.primary == nil {
		return nil
	}
	own := map[string]bool{}
	for _, r := range c.primary.Records {
		own[r.URL] = true
	}
	// who uses each picture
	byPhoto := map[string][]*models.Record{}
	for i := range c.hits {
		r := c.hits[i].Record
		if r != nil && r.Photo != "" {
			byPhoto[r.Photo] = append(byPhoto[r.Photo], r)
		}
	}
	var out []models.AccountFlag
	for i := range c.hits {
		f := c.hits[i]
		r := f.Record
		if r == nil || !own[r.URL] {
			continue
		}
		var fl []string
		if strings.TrimSpace(r.Name) == "" {
			fl = append(fl, "no name on the profile")
		}
		if strings.TrimSpace(r.Bio) == "" && r.Photo == "" {
			fl = append(fl, "empty profile: no bio and no picture")
		}
		for _, o := range byPhoto[r.Photo] {
			if r.Photo != "" && o != r && o.Name != "" && r.Name != "" && !strings.EqualFold(o.Name, r.Name) {
				fl = append(fl, "picture also used under a different name ("+o.Name+" on "+o.Platform+")")
				break
			}
		}
		if t, ok := parseDate(f.Details["created"]); ok && time.Since(t) < 90*24*time.Hour {
			fl = append(fl, "account created in the last 90 days")
		}
		if r.Self {
			fl = append(fl, "written by the person")
		}
		if len(fl) > 0 {
			out = append(out, models.AccountFlag{Platform: r.Platform, URL: r.URL, Flags: fl})
		}
	}
	return out
}
