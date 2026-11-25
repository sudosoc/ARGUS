// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

type acct struct {
	label, url, platform string
	handle               string
	bio                  string
	values               map[string]bool // e-mails, phones, wallets, profile links the account states
}

// hiddenLinks finds accounts that nothing stated as linked but that share something distinctive: a handle
// pattern, a distinctive bio, a value (e-mail, phone, wallet, website) or a near-identical picture.
func hiddenLinks(c *ctx) []models.HiddenLink {
	var accts []*acct
	seen := map[string]bool{}
	for i := range c.hits {
		f := c.hits[i]
		r := f.Record
		a := &acct{values: map[string]bool{}}
		switch {
		case r != nil:
			a.label, a.url, a.platform, a.bio = r.Platform+": "+firstNonEmpty(r.Name, f.Source), r.URL, r.Platform, r.Bio
			a.handle = handleOf(r)
			for _, e := range r.Emails {
				a.values["email:"+strings.ToLower(e)] = true
			}
			for _, p := range r.Phones {
				a.values["phone:"+p] = true
			}
			for _, w := range r.Websites {
				if h := hostOf(w); h != "" && !commonHost[h] {
					a.values["site:"+h] = true
				}
			}
		case f.Module == "social" && f.Status == models.StatusFound:
			a.label, a.url, a.platform = f.Source, f.URL, strings.TrimSuffix(f.Source, " profile")
			a.handle = strings.ToLower(f.Details["username"])
			a.bio = f.Details["bio"]
		default:
			continue
		}
		k := a.url
		if k == "" {
			k = a.label
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		accts = append(accts, a)
	}
	if len(accts) > 400 {
		accts = accts[:400]
	}

	var out []models.HiddenLink
	pair := func(a, b *acct, kind string, strength float64, ev string) {
		if a.url != "" && a.url == b.url {
			return
		}
		out = append(out, models.HiddenLink{A: a.label, B: b.label, Kind: kind, Strength: float64(int(strength*100)) / 100, Evidence: []string{ev}})
	}

	// handle patterns: the same stem with different decoration (lhassan / l.hassan / lhassan89)
	byStem := map[string][]*acct{}
	for _, a := range accts {
		if s := stem(a.handle); len(s) >= 5 {
			byStem[s] = append(byStem[s], a)
		}
	}
	for s, list := range byStem {
		// one representative account per distinct handle: the pairs that matter are across different spellings
		reps := map[string]*acct{}
		var order []string
		for _, a := range list {
			h := strings.ToLower(a.handle)
			if reps[h] == nil {
				reps[h] = a
				order = append(order, h)
			}
		}
		if len(order) < 2 { // identical handles are already "the same handle" and not hidden
			continue
		}
		for i := 0; i < len(order) && i < 8; i++ {
			for j := i + 1; j < len(order) && j < 8; j++ {
				pair(reps[order[i]], reps[order[j]], "handle pattern", 0.55, fmt.Sprintf("“%s” and “%s” share the core “%s”", order[i], order[j], s))
			}
		}
	}

	// text similarity: the same distinctive sentences in two bios
	shingles := make([]map[string]bool, len(accts))
	for i, a := range accts {
		shingles[i] = shingle(a.bio)
	}
	for i := 0; i < len(accts); i++ {
		for j := i + 1; j < len(accts); j++ {
			if len(shingles[i]) < 4 || len(shingles[j]) < 4 {
				continue
			}
			if sim := jaccard(shingles[i], shingles[j]); sim >= 0.45 {
				pair(accts[i], accts[j], "similar text", 0.5+0.4*sim, fmt.Sprintf("their bios share %d%% of their wording", int(sim*100)))
			}
		}
	}

	// shared values
	byValue := map[string][]*acct{}
	for _, a := range accts {
		for v := range a.values {
			byValue[v] = append(byValue[v], a)
		}
	}
	for v, list := range byValue {
		if len(list) < 2 || len(list) > 6 {
			continue
		}
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if list[i].platform != list[j].platform {
					pair(list[i], list[j], "shared value", 0.85, "both state "+strings.Replace(v, ":", " ", 1))
				}
			}
		}
	}

	// near-identical pictures that are not exact copies
	type ph struct {
		h       uint64
		sources []string
		group   int
		url     string
	}
	var phs []ph
	for _, p := range c.rep.Photos {
		if p.DHash == "" {
			continue
		}
		h, err := strconv.ParseUint(p.DHash, 16, 64)
		if err != nil {
			continue
		}
		phs = append(phs, ph{h, p.Sources, p.Group, p.URL})
	}
	for i := 0; i < len(phs) && i < 300; i++ {
		for j := i + 1; j < len(phs) && j < 300; j++ {
			d := bits.OnesCount64(phs[i].h ^ phs[j].h)
			if d == 0 || d > 10 || (phs[i].group != 0 && phs[i].group == phs[j].group) {
				continue
			}
			out = append(out, models.HiddenLink{A: strings.Join(phs[i].sources, ", "), B: strings.Join(phs[j].sources, ", "), Kind: "similar picture",
				Strength: float64(int((0.9-float64(d)*0.04)*100)) / 100, Evidence: []string{fmt.Sprintf("the pictures differ in %d of 64 fingerprint bits: the same photo cropped, resized or recoloured", d)}})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Strength > out[j].Strength })
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func handleOf(r *models.Record) string {
	for _, u := range r.Usernames {
		if i := strings.Index(u, ":"); i >= 0 && strings.EqualFold(u[:i], r.Platform) {
			return strings.ToLower(u[i+1:])
		}
	}
	if len(r.Usernames) > 0 {
		u := r.Usernames[0]
		if i := strings.Index(u, ":"); i >= 0 {
			u = u[i+1:]
		}
		return strings.ToLower(u)
	}
	return ""
}

var commonHost = map[string]bool{"github.com": true, "twitter.com": true, "x.com": true, "linkedin.com": true, "facebook.com": true, "instagram.com": true,
	"youtube.com": true, "medium.com": true, "gitlab.com": true, "google.com": true, "wikipedia.org": true, "t.me": true, "reddit.com": true}

func hostOf(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	u = strings.TrimPrefix(u, "www.")
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// shingle turns a text into its set of overlapping word triples.
func shingle(s string) map[string]bool {
	ws := words(s)
	out := map[string]bool{}
	for i := 0; i+2 < len(ws); i++ {
		out[ws[i]+" "+ws[i+1]+" "+ws[i+2]] = true
	}
	return out
}
