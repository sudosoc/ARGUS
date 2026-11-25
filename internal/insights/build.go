// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/sensitive"
)

// ctx is what every rule reads.
type ctx struct {
	withheld int // derived items left out because they concern a protected category (package sensitive)
	rep      *models.Report
	primary  *models.Identity // the best-matching person, nil when none could be resolved
	hits     []models.Finding
}

// Build derives the insights of a finished report. It never fails and never changes the report.
func Build(rep *models.Report) *models.Insights {
	c := &ctx{rep: rep, hits: rep.Hits()}
	for i := range rep.Identities {
		if !rep.Identities[i].Weak && (rep.Identities[i].Primary || c.primary == nil) {
			c.primary = &rep.Identities[i]
			if rep.Identities[i].Primary {
				break
			}
		}
	}
	in := &models.Insights{}
	in.Persona = persona(c)
	in.Authenticity = authenticity(c)
	in.Associates = associates(c)
	in.HiddenLinks = hiddenLinks(c)
	in.Life = life(c)
	in.Movement = movement(c)
	in.Explain = explain(c)
	in.Brief = brief(c, in)
	in.Actions = actions(c, in)
	in.Limits = limits(c, in)
	// Last line of defence: nothing derived may state a protected characteristic.
	c.withheld += sensitive.Scrub(in)
	if c.withheld > 0 {
		in.Limits = append([]string{fmt.Sprintf("%d derived item(s) were left out because they concern religion or belief, health, sexual orientation or gender identity, or political or trade-union affiliation. ARGUS never analyses these, even when they appear in the evidence.", c.withheld)}, in.Limits...)
	}
	return in
}

// limits states what the evidence did not allow, so an empty section is explained and not mistaken for "nothing exists".
func limits(c *ctx, in *models.Insights) []string {
	var l []string
	if c.primary == nil {
		l = append(l, "No single person could be resolved, so the persona, authenticity and explanations describe no one in particular.")
	}
	if in.Persona == nil || in.Persona.Schedule == nil {
		l = append(l, "No daily rhythm: it needs a username with public activity (GitHub, Hacker News, Reddit, Wikipedia…).")
	}
	if in.Movement == nil || len(in.Movement.Places) == 0 {
		l = append(l, "No movement: it needs pictures that carry GPS and a date, or dated location evidence.")
	}
	if len(in.Associates) == 0 {
		l = append(l, "No associates: they come from public follow lists (Bluesky), organisations and schools stated in profiles; none was found.")
	}
	l = append(l, "Insights are rules applied to public evidence. They are leads to check, not conclusions about a person.")
	return l
}
