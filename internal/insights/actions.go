// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// actions lists what to do next, most valuable first. Each one says why it matters and what it would improve.
func actions(c *ctx, in *models.Insights) []models.Action {
	var out []models.Action
	add := func(prio int, kind, title, why, how, gain, url string) {
		out = append(out, models.Action{Priority: prio, Kind: kind, Title: title, Why: why, How: how, Gain: gain, URL: url})
	}
	t := c.rep.Target
	realIDs := 0
	for _, id := range c.rep.Identities {
		if !id.Weak {
			realIDs++
		}
	}

	// several people match: narrow
	if c.rep.Ambiguity != "" || realIDs >= 2 {
		if strings.TrimSpace(t.Location) == "" {
			add(92, "narrow", "Add a location", fmt.Sprintf("%d different people match what you supplied, and a place is the quickest way to tell them apart.", max(realIDs, 2)), "Enter a city or country in the form and run again (or use the “Search again with these details” button on a candidate).", "separates the people who share the name", "")
		}
		if strings.TrimSpace(t.Company) == "" {
			add(84, "narrow", "Add an employer or school", "Profiles that mention it rank first, which pushes wrong people out.", "Enter the organisation in “Company / organisation”, or paste the person's CV into “Paste text”.", "raises the match score of the right person", "")
		}
		if t.Email == "" && t.Username == "" {
			add(80, "narrow", "Add an e-mail address or a username", "An e-mail or a handle ties records to one person far better than a name.", "Fill “E-mail” or “Username / handle” in the form.", "turns name guesses into confirmed accounts", "")
		}
	}

	// sanctions
	for _, f := range c.hits {
		if f.Module == "sanctions" {
			add(96, "verify", "Compare the possible watch-list match by hand", "A name matched an official sanctions or watch list. A name match is not an identification.", "Compare date of birth, nationality and the programme on the official entry with what you know.", "avoids a false accusation", f.URL)
			break
		}
	}

	// picture reuse under a different name
	if a := in.Authenticity; a != nil {
		for _, af := range a.Accounts {
			for _, fl := range af.Flags {
				if strings.HasPrefix(fl, "picture also used under a different name") {
					add(88, "verify", "Check for an impersonator", fmt.Sprintf("The %s account uses a picture that appears under another name (%s).", af.Platform, strings.TrimPrefix(fl, "picture also used under a different name ")), "Open both profiles and compare creation dates, activity and followers: the older, more active one is usually the original.", "removes a fake account from the picture", af.URL)
					break
				}
			}
		}
		if a.Score < 55 {
			add(78, "verify", "Verify this is a real, established person", "The identity looks "+strings.ToLower(a.Label)+" ("+fmt.Sprint(a.Score)+"/100).", "Look for independent mentions (news, registers, research databases), check how old the accounts are and run a reverse image search on the profile picture.", "separates a real person from a thin or fabricated identity", "")
		}
	}

	// contradictions
	if c.primary != nil {
		for _, cf := range c.primary.Conflicts {
			add(72, "resolve", "Resolve: "+cf, "Sources state different values for the same attribute: it may be a move, an old profile or two people.", "Open Findings → Conflicting findings, read who says what and when each profile was last active.", "makes the profile consistent", "")
		}
	}

	// hidden links
	for _, hl := range in.HiddenLinks {
		if hl.Strength >= 0.7 {
			add(66, "verify", "Confirm the link between two accounts", fmt.Sprintf("“%s” and “%s” look connected (%s) though nothing says so.", hl.A, hl.B, hl.Kind), "Open both and compare names, pictures, writing and dates; then add one of them as a username and run again.", "may reveal another account of the same person", "")
			break
		}
	}

	// manual leads for what was supplied
	order := []string{"name", "email", "phone", "username", "image", "domain", "company"}
	picked := 0
	for _, g := range order {
		for _, l := range c.rep.ManualLeads {
			if l.Group == g && picked < 3 {
				add(58-picked*2, "collect", "Open "+l.Label, l.Why, "Open the link: it needs a login, a CAPTCHA or payment, so ARGUS does not automate it.", "reaches a source ARGUS cannot query", l.URL)
				picked++
				break
			}
		}
	}

	// coverage
	if checks, errs := coverage(c.rep); checks > 0 && float64(errs)/float64(checks) > 0.15 {
		add(62, "rerun", "Re-run later or through a proxy", fmt.Sprintf("%d of %d checks failed or were blocked.", errs, checks), "Run again with -proxy-file, or later in the day; see Evidence → Collection coverage for which sources failed.", "fills coverage gaps that look like “nothing found”", "")
	}

	// unused inputs unlock collectors
	missing := map[string]string{
		"E-mail":    "unlocks breach checks, Gravatar, PGP keys and public-code search",
		"Domain":    "unlocks DNS, subdomains, TLS, website technology and archives",
		"Phone":     "unlocks number analysis (country, carrier, line type)",
		"Username":  "unlocks the 10,600-site account search and activity hours",
		"Full name": "unlocks person records, sanctions screening and press mentions",
	}
	have := map[string]bool{"E-mail": t.Email != "", "Domain": t.Domain != "", "Phone": t.Phone != "", "Username": t.Username != "" || len(t.Aliases) > 0, "Full name": t.FullName != ""}
	var gaps []string
	for _, k := range []string{"Username", "E-mail", "Full name", "Domain", "Phone"} {
		if !have[k] {
			gaps = append(gaps, k)
		}
	}
	if len(gaps) > 0 && len(c.hits) > 0 {
		k := gaps[0]
		add(48, "collect", "Add a "+strings.ToLower(k)+" if you know one", k+" "+missing[k]+".", "Enter it in the form and run again; ARGUS then follows it too.", "more collectors run", "")
	}

	// defend: what the person's own habits give away
	if p := in.Persona; p != nil && len(p.Exposure) >= 2 {
		add(52, "defend", "Reduce what the person gives away", p.Exposure[0]+" (and "+fmt.Sprint(len(p.Exposure)-1)+" more).", "Use a private commit e-mail (the platform's noreply address), vary handles between accounts, strip EXIF/GPS before sharing pictures, and enable two-factor authentication.", "makes the person harder to profile", "")
	}

	// infrastructure risks (for domain targets)
	for _, f := range c.hits {
		if f.Category == "exposure" || f.Category == "takeover" {
			add(70, "defend", "Fix: "+f.Source, firstNonEmpty(f.Details["note"], f.Details["path"], "an exposed or takeover-prone asset was found"), "See Overview → Risk signals for the recommended action; fix it and run again to confirm.", "closes a real exposure", f.URL)
		}
	}
	if len(c.hits) == 0 {
		add(90, "collect", "Give ARGUS more to work with", "The run found nothing.", "Add a username, an e-mail, a domain or a full name plus a location, and check Evidence → Collection coverage for checks that could not run.", "gets the first findings", "")
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	// drop duplicates by title
	seen := map[string]bool{}
	var dedup []models.Action
	for _, a := range out {
		if !seen[a.Title] {
			seen[a.Title] = true
			dedup = append(dedup, a)
		}
	}
	if len(dedup) > 12 {
		dedup = dedup[:12]
	}
	return dedup
}

func coverage(rep *models.Report) (checks, errs int) {
	for _, m := range rep.Modules {
		checks += m.Checked
		errs += m.Errors
	}
	return
}
