// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// brief writes the one-screen summary from the other insights and the evidence.
func brief(c *ctx, in *models.Insights) models.Brief {
	b := models.Brief{}
	P := c.primary
	nHits := len(c.hits)
	if P == nil {
		b.Level = "none"
		b.Headline = "No single person could be identified from the evidence"
		if nHits == 0 {
			b.Paragraphs = []string{"The investigation produced no positive findings. Check Evidence → Collection coverage for checks that could not run, and add more identifiers."}
		} else {
			b.Paragraphs = []string{fmt.Sprintf("%s were found, but they do not add up to one identifiable person. They are leads about accounts, hosts or documents, not about an individual.", plural(nHits, "finding", "findings"))}
			if c.rep.Ambiguity != "" {
				b.Concerns = append(b.Concerns, c.rep.Ambiguity)
			}
		}
		return b
	}

	name := firstNonEmpty(bestFact(P.Names), P.Label)
	head := bestFact(P.Headlines)
	place := ""
	if c.rep.Geo != nil {
		place = c.rep.Geo.Consensus
	}
	if place == "" {
		place = bestFact(P.Locations)
	}
	parts := []string{name}
	if head != "" {
		parts = append(parts, head)
	}
	if place != "" {
		parts = append(parts, place)
	}
	b.Headline = strings.Join(parts, " · ")
	switch {
	case P.Score >= 0.75 && c.rep.Ambiguity == "":
		b.Level = "strong"
	case P.Score >= 0.5:
		b.Level = "moderate"
	default:
		b.Level = "weak"
	}

	confirmed, total := 0, 0
	for _, fs := range [][]models.Fact{P.Names, P.Headlines, P.Employers, P.Locations, P.Education, P.Emails, P.Phones, P.Usernames, P.Websites} {
		for _, f := range fs {
			total++
			if f.Confirmed {
				confirmed++
			}
		}
	}
	lvl := map[string]string{"strong": "a strong", "moderate": "a moderate", "weak": "a weak"}[b.Level]
	p1 := fmt.Sprintf("The best match is %s, %s match (%d%%) with what was supplied, described by %s.", name, lvl, int(P.Score*100), plural(len(P.Records), "source", "sources"))
	if confirmed > 0 {
		p1 += fmt.Sprintf(" %d of %d facts are confirmed by two or more independent platforms.", confirmed, total)
	}
	b.Paragraphs = append(b.Paragraphs, p1)

	if per := in.Persona; per != nil {
		var bits []string
		if per.FirstTrace != "" {
			s := "first seen online in " + per.FirstTrace[:4]
			if per.FirstSource != "" {
				s += " (" + per.FirstSource + ")"
			}
			bits = append(bits, s)
		}
		if len(per.Languages) > 0 {
			var ls []string
			for _, l := range per.Languages {
				ls = append(ls, l.Label)
			}
			bits = append(bits, "writes in "+strings.Join(ls, " and "))
		}
		if len(per.Expertise) > 0 {
			var xs []string
			for i, x := range per.Expertise {
				if i == 3 {
					break
				}
				xs = append(xs, x.Label)
			}
			bits = append(bits, "focuses on "+strings.Join(xs, ", "))
		}
		if per.Schedule != nil {
			bits = append(bits, fmt.Sprintf("is %s (time zone %s)", per.Schedule.Pattern, per.Schedule.Zone))
		}
		if len(bits) > 0 {
			b.Paragraphs = append(b.Paragraphs, "The person "+strings.Join(bits, "; ")+".")
		}
	}
	if a := in.Authenticity; a != nil {
		b.Paragraphs = append(b.Paragraphs, fmt.Sprintf("The identity looks %s (%d/100): %s.", strings.ToLower(a.Label), a.Score, topFactors(a)))
	}
	if len(in.Associates) > 0 {
		var ns []string
		for i, x := range in.Associates {
			if i == 3 {
				break
			}
			ns = append(ns, x.Name)
		}
		b.Paragraphs = append(b.Paragraphs, "Closest connections: "+strings.Join(ns, ", ")+".")
	}
	if mv := in.Movement; mv != nil && mv.Home != "" {
		s := "The evidence places the person around " + mv.Home + "."
		if len(mv.Trips) > 0 {
			s += fmt.Sprintf(" %s between places were derived from dated pictures.", plural(len(mv.Trips), "move", "moves"))
		}
		b.Paragraphs = append(b.Paragraphs, s)
	}

	// strengths and concerns
	if confirmed >= 3 {
		b.Strengths = append(b.Strengths, fmt.Sprintf("%d facts confirmed by independent platforms", confirmed))
	}
	for _, f := range c.hits {
		if independent[f.Module] && f.Record != nil {
			b.Strengths = append(b.Strengths, "Mentioned in third-party records (registers, research databases)")
			break
		}
	}
	if a := in.Authenticity; a != nil && a.Score >= 75 {
		b.Strengths = append(b.Strengths, "Long-lived, consistent and independently corroborated identity")
	}
	if c.rep.Ambiguity != "" {
		b.Concerns = append(b.Concerns, c.rep.Ambiguity)
	}
	if n := len(P.Conflicts); n > 0 {
		b.Concerns = append(b.Concerns, fmt.Sprintf("Sources disagree about: %s", strings.Join(P.Conflicts, ", ")))
	}
	if a := in.Authenticity; a != nil && a.Score < 55 {
		b.Concerns = append(b.Concerns, "The identity looks "+strings.ToLower(a.Label)+": "+topFactors(a))
	}
	if n := len(c.rep.Failures); n > 0 && len(c.rep.Modules) > 0 {
		checks := 0
		for _, m := range c.rep.Modules {
			checks += m.Checked
		}
		if checks > 0 && float64(n)/float64(checks) > 0.1 {
			b.Concerns = append(b.Concerns, fmt.Sprintf("%d of %d checks could not complete: absence of a result is not proof of absence", n, checks))
		}
	}
	if len(in.HiddenLinks) > 0 {
		b.Concerns = append(b.Concerns, fmt.Sprintf("%s between accounts that nothing stated as linked: verify them", plural(len(in.HiddenLinks), "possible link", "possible links")))
	}
	return b
}

func bestFact(fs []models.Fact) string {
	best := ""
	bn := -1
	for _, f := range fs {
		n := len(f.Sources)
		if f.Confirmed {
			n += 10
		}
		if f.Self {
			n += 2
		}
		if n > bn {
			best, bn = f.Value, n
		}
	}
	return best
}

func topFactors(a *models.Authenticity) string {
	var parts []string
	for i, f := range a.Factors {
		if i == 3 || f.Effect == 0 {
			continue
		}
		sign := "+"
		if f.Effect < 0 {
			sign = "−"
		}
		parts = append(parts, fmt.Sprintf("%s (%s%d)", strings.ToLower(f.Name), sign, abs(f.Effect)))
	}
	if len(parts) == 0 {
		return "no strong signal either way"
	}
	return "driven by " + strings.Join(parts, ", ")
}
