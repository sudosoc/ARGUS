// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"fmt"
	"math/bits"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// Compare weighs the evidence that two resolved people are one individual. It exists because the resolver is
// deliberately cautious: it never merges two accounts on the same platform, so a person's second account, or an
// alias with a different name, stays a separate candidate. Compare shows how much they have in common (a similar
// picture, a shared first name, one handle inside the other, a shared e-mail or site) and what speaks against.
//
// The score combines independent signals (each one a probability that it alone would be this convincing) and is
// lowered by contrary evidence. It is a lead to check, never a verdict: a copied picture and name is also what an
// impersonator looks like, and the advice says how to tell.
func Compare(a, b models.Identity, photos []models.Photo) models.Comparison {
	c := models.Comparison{A: a.ID, B: b.ID, ALabel: a.Label, BLabel: b.Label}
	forP, againstP := 1.0, 1.0
	add := func(name, effect string, w float64, detail string) {
		c.Signals = append(c.Signals, models.Signal{Name: name, Effect: effect, Weight: w, Detail: detail})
		switch effect {
		case "for":
			forP *= 1 - w
		case "against":
			againstP *= 1 - w
		}
	}

	// ---- pictures
	if pp, ok := closestPhotos(a, b, photos); ok {
		c.Photos = []models.PhotoPair{pp}
		switch pp.Kind {
		case "identical":
			add("Same picture", "for", 0.7, fmt.Sprintf("the same picture is used by both (%d of 64 fingerprint bits differ)", pp.Distance))
		case "near":
			add("Very similar picture", "for", 0.5, fmt.Sprintf("the pictures are almost the same: a crop, a resize or a recolour (%d of 64 bits differ)", pp.Distance))
		default:
			add("Similar picture", "for", 0.25, fmt.Sprintf("the pictures look alike (%d of 64 bits differ): the same scene or person, or only a similar style", pp.Distance))
		}
	}

	// ---- names
	best, shared := nameRelation(a, b)
	switch best {
	case NameStrong:
		add("Same name", "for", 0.45, "the names match (order, accents, initials and spelling ignored)")
	case NamePartial:
		add("Similar name", "for", 0.3, "the names partly match")
	default:
		if shared != "" {
			add("Shared name part", "for", 0.15, "both contain “"+shared+"”: common in a family, a team or a nickname, so a weak sign")
		} else if hasFullName(a) && hasFullName(b) {
			add("Different full names", "against", 0.35, "each states a full name and they share no part")
		}
	}

	// ---- handles
	ha, hb := handles(a), handles(b)
	if h := commonHandle(ha, hb); h != "" {
		add("Same handle", "for", 0.5, "both use the handle “"+h+"”")
	} else if r := relatedHandles(ha, hb); r != "" {
		add("Related handles", "for", 0.25, r)
	} else if r := handleInName(ha, hb, a, b); r != "" {
		add("Handle in the other's name", "for", 0.3, r)
	}

	// ---- values both state
	if v := sharedFact(a.Emails, b.Emails); v != "" {
		add("Same e-mail address", "for", 0.9, v+" appears on both")
	}
	if v := sharedFact(a.Phones, b.Phones); v != "" {
		add("Same phone number", "for", 0.9, v+" appears on both")
	}
	if v := sharedSite(a, b); v != "" {
		add("Same personal site", "for", 0.5, "both link to "+v)
	}
	if v := sharedFact(a.Employers, b.Employers); v != "" {
		add("Same employer", "for", 0.2, v)
	}
	if v := sharedFact(a.Education, b.Education); v != "" {
		add("Same school", "for", 0.18, v)
	}
	if v := sharedFact(a.Locations, b.Locations); v != "" {
		add("Same place", "for", 0.12, v)
	}
	if bioSimilar(a.Bio, b.Bio) {
		add("Same bio text", "for", 0.35, "the descriptions are the same text or nearly")
	}
	if v := sharedFact(a.Headlines, b.Headlines); v != "" {
		add("Same headline", "for", 0.15, v)
	}
	for k, v := range a.IDs {
		if w, ok := b.IDs[k]; ok {
			if strings.EqualFold(v, w) {
				add("Same "+k+" identifier", "for", 0.95, k+" "+v)
			} else {
				add("Different "+k+" identifiers", "against", 0.9, k+": "+v+" vs "+w)
			}
		}
	}

	// ---- two accounts on one platform
	if p := samePlatform(a, b); p != "" {
		c.Signals = append(c.Signals, models.Signal{Name: "Both have a " + p + " account", Effect: "note",
			Detail: "a person usually has one, so this is a second account of the same person, a different person, or a copy; the resolver keeps them apart for that reason"})
	}

	c.Score = float64(int((1-forP)*againstP*100+0.5)) / 100
	switch {
	case c.Score >= 0.8:
		c.Verdict = "same"
	case c.Score >= 0.55:
		c.Verdict = "likely"
	case c.Score >= 0.3:
		c.Verdict = "possible"
	default:
		c.Verdict = "unlikely"
	}
	sort.SliceStable(c.Signals, func(i, j int) bool {
		rank := map[string]int{"for": 0, "against": 1, "note": 2}
		if rank[c.Signals[i].Effect] != rank[c.Signals[j].Effect] {
			return rank[c.Signals[i].Effect] < rank[c.Signals[j].Effect]
		}
		return c.Signals[i].Weight > c.Signals[j].Weight
	})
	c.Advice = advice(c, a, b)
	return c
}

// Link records, on each of the first identities, the others that may be the same individual. Nothing is merged.
func Link(ids []models.Identity, photos []models.Photo) {
	n := len(ids)
	if n > 14 {
		n = 14
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if ids[i].Weak && ids[j].Weak {
				continue
			}
			c := Compare(ids[i], ids[j], photos)
			if c.Score < 0.3 {
				continue
			}
			why := []string{}
			for _, s := range c.Signals {
				if s.Effect == "for" && s.Weight >= 0.15 {
					why = append(why, strings.ToLower(s.Name[:1])+s.Name[1:])
				}
			}
			ids[i].Related = append(ids[i].Related, models.Related{ID: ids[j].ID, Label: ids[j].Label, Verdict: c.Verdict, Score: c.Score, Why: why})
			ids[j].Related = append(ids[j].Related, models.Related{ID: ids[i].ID, Label: ids[i].Label, Verdict: c.Verdict, Score: c.Score, Why: why})
		}
	}
	for i := range ids {
		sort.SliceStable(ids[i].Related, func(x, y int) bool { return ids[i].Related[x].Score > ids[i].Related[y].Score })
		if len(ids[i].Related) > 3 {
			ids[i].Related = ids[i].Related[:3]
		}
	}
}

// ------------------------------------------------------------------ pieces

func closestPhotos(a, b models.Identity, photos []models.Photo) (models.PhotoPair, bool) {
	hash := map[string]uint64{}
	group := map[string]int{}
	for _, p := range photos {
		if len(p.DHash) == 16 {
			if v, err := strconv.ParseUint(p.DHash, 16, 64); err == nil {
				hash[p.URL] = v
			}
		}
		if p.Group > 0 {
			group[p.URL] = p.Group
		}
	}
	best := models.PhotoPair{Distance: 65}
	for _, x := range a.Photos {
		for _, y := range b.Photos {
			if x == y || (group[x] > 0 && group[x] == group[y]) {
				return models.PhotoPair{A: x, B: y, Distance: 0, Kind: "identical"}, true
			}
			hx, ok1 := hash[x]
			hy, ok2 := hash[y]
			if !ok1 || !ok2 {
				continue
			}
			if d := bits.OnesCount64(hx ^ hy); d < best.Distance {
				best = models.PhotoPair{A: x, B: y, Distance: d}
			}
		}
	}
	switch {
	case best.Distance <= 6:
		best.Kind = "identical"
	case best.Distance <= 12:
		best.Kind = "near"
	case best.Distance <= 18:
		best.Kind = "similar"
	default:
		return models.PhotoPair{}, false
	}
	return best, true
}

func nameRelation(a, b models.Identity) (NameMatch, string) {
	best := NameNone
	for _, x := range a.Names {
		for _, y := range b.Names {
			if m := MatchNames(x.Value, y.Value); m > best {
				best = m
			}
		}
	}
	if best != NameNone {
		return best, ""
	}
	common := map[string]bool{"mohamed": true, "muhammad": true, "mohammed": true, "ahmed": true, "john": true, "david": true, "michael": true, "ali": true, "user": true, "admin": true}
	set := map[string]bool{}
	for _, x := range a.Names {
		for _, w := range nameTokens(x.Value) {
			if len(w) >= 4 && !common[w] {
				set[w] = true
			}
		}
	}
	for _, y := range b.Names {
		for _, w := range nameTokens(y.Value) {
			if set[w] {
				return NameNone, w
			}
		}
	}
	return NameNone, ""
}

func hasFullName(id models.Identity) bool {
	for _, n := range id.Names {
		if len(nameTokens(n.Value)) >= 2 {
			return true
		}
	}
	return false
}

// handles returns the lower-cased handles of an identity without the platform prefix.
func handles(id models.Identity) []string {
	var out []string
	for _, u := range id.Usernames {
		h := u.Value
		if _, rest, ok := strings.Cut(h, ":"); ok {
			h = rest
		}
		h = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "@"))
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}

func commonHandle(a, b []string) string {
	set := map[string]bool{}
	for _, h := range a {
		set[h] = true
	}
	for _, h := range b {
		if set[h] && len(h) >= 4 {
			return h
		}
	}
	return ""
}

func trigrams(s string) map[string]bool {
	out := map[string]bool{}
	r := []rune(s)
	for i := 0; i+3 <= len(r); i++ {
		out[string(r[i:i+3])] = true
	}
	return out
}

// relatedHandles reports two handles that are not equal but clearly built from each other (one inside the other, or
// mostly the same letters).
func relatedHandles(a, b []string) string {
	for _, x := range a {
		for _, y := range b {
			if x == y || len(x) < 5 || len(y) < 5 {
				continue
			}
			if strings.Contains(x, y) || strings.Contains(y, x) {
				return "“" + x + "” and “" + y + "”: one contains the other"
			}
			tx, ty := trigrams(x), trigrams(y)
			inter := 0
			for g := range tx {
				if ty[g] {
					inter++
				}
			}
			if u := len(tx) + len(ty) - inter; u > 0 && float64(inter)/float64(u) >= 0.6 {
				return "“" + x + "” and “" + y + "” are mostly the same letters"
			}
		}
	}
	return ""
}

// handleInName reports a handle of one person that appears in a name the other uses (“Seif Alsayed sudosoc”).
func handleInName(ha, hb []string, a, b models.Identity) string {
	check := func(hs []string, other models.Identity, who string) string {
		for _, h := range hs {
			if len(h) < 5 {
				continue
			}
			for _, n := range other.Names {
				for _, w := range nameTokens(n.Value) {
					if w == h {
						return who + "’s handle “" + h + "” is part of a name the other uses"
					}
				}
			}
		}
		return ""
	}
	if s := check(ha, b, "first person"); s != "" {
		return s
	}
	return check(hb, a, "second person")
}

func sharedFact(a, b interface{}) string {
	list := func(v interface{}) []string {
		switch t := v.(type) {
		case []models.Fact:
			var o []string
			for _, f := range t {
				o = append(o, f.Value)
			}
			return o
		}
		return nil
	}
	la, lb := list(a), list(b)
	for _, x := range la {
		for _, y := range lb {
			if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(y)) || overlap(x, y) {
				return x
			}
		}
	}
	return ""
}

func sharedSite(a, b models.Identity) string {
	host := func(u string) string {
		if !strings.Contains(u, "://") {
			u = "https://" + u
		}
		p, err := url.Parse(u)
		if err != nil {
			return ""
		}
		return strings.TrimPrefix(strings.ToLower(p.Hostname()), "www.")
	}
	set := map[string]bool{}
	for _, w := range a.Websites {
		if h := host(w.Value); h != "" && !platformHosts[h] {
			set[h] = true
		}
	}
	for _, w := range b.Websites {
		if h := host(w.Value); h != "" && set[h] {
			return h
		}
	}
	return ""
}

func samePlatform(a, b models.Identity) string {
	set := map[string]bool{}
	for _, r := range a.Records {
		set[strings.ToLower(r.Platform)] = true
	}
	for _, r := range b.Records {
		p := strings.ToLower(r.Platform)
		switch p {
		case "github", "gitlab", "linkedin", "twitter", "x", "mastodon", "bluesky", "stack overflow", "reddit", "keybase":
			if set[p] {
				return r.Platform
			}
		}
	}
	return ""
}

func advice(c models.Comparison, a, b models.Identity) []string {
	var out []string
	hasPhoto := len(c.Photos) > 0
	if hasPhoto {
		out = append(out, "Put the two pictures side by side and look at them yourself: a similar picture on two accounts is the same person’s second account, or a copy made by someone else.")
	}
	if samePlatform(a, b) != "" {
		out = append(out, "Open both accounts on the shared platform: compare when each was created, what each posts and who follows each. A copy is usually newer, quieter and has no history.")
	}
	if c.Verdict == "possible" || c.Verdict == "likely" {
		out = append(out, "Look for something only the person would do: a link from one account to the other, the same bio sentence, the same photo in different years.")
		out = append(out, "If you are the person, or you know them, the quickest way to settle it is to add the other account’s username to the inputs and run again.")
	}
	if len(c.Signals) == 0 {
		out = append(out, "Nothing in common was found. Add an e-mail, a username or a place for either person and run again.")
	}
	return out
}
