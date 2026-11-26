// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/sudosoc/argus/internal/models"
)

// authored reports whether the item is the person's own writing (not a boost, a star or a repository).
func authored(p models.Post) bool {
	switch p.Kind {
	case "post", "reply", "comment", "issue", "pull", "commit":
		return true
	}
	return false
}

// writing reports whether the item is prose (commit messages and repository blurbs are not).
func writing(p models.Post) bool {
	switch p.Kind {
	case "post", "reply", "comment", "issue", "pull":
		return strings.TrimSpace(p.Text) != ""
	}
	return false
}

// content summarises the copied public posts. It is nil when nothing was copied and nothing was asked for.
func content(c *ctx) *models.Content {
	ps := c.rep.Posts
	if len(ps) == 0 && len(c.rep.PostSources) == 0 {
		return nil
	}
	out := &models.Content{Total: len(ps), Sources: c.rep.PostSources}
	if len(ps) == 0 {
		out.Notes = append(out.Notes, "No public items could be copied; each place's note says why.")
		return out
	}

	self := map[string]bool{}
	for _, s := range c.rep.PostSources {
		self[strings.ToLower(s.Handle)] = true
		if i := strings.IndexByte(s.Handle, '@'); i > 0 {
			self[strings.ToLower(s.Handle[:i])] = true
		}
	}

	kinds := map[string]int{}
	months := map[string]int{}
	var texts []string
	topics := map[string]int{}
	inter := map[string]*[3]int{} // replies, mentions, boosts
	domains := map[string]int{}
	var words, linkN, mediaN, questN, emojiN, shoutN, replyN, proseN int
	for _, p := range ps {
		kinds[p.Kind]++
		if len(p.Time) >= 7 {
			months[p.Time[:7]]++
		}
		if out.First == "" || (p.Time != "" && p.Time < out.First) {
			out.First = p.Time
		}
		if p.Time > out.Last {
			out.Last = p.Time
		}
		for _, t := range p.Tags {
			if t != "" && !c.protected(t) {
				topics[t]++
			}
		}
		for _, l := range p.Links {
			if u, err := url.Parse(l); err == nil && u.Hostname() != "" {
				h := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
				if !c.protected(h) {
					domains[h]++
				}
			}
		}
		bump := func(h string, i int) {
			h = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), "@")
			if h == "" || self[h] || c.protected(h) {
				return
			}
			e := inter[h]
			if e == nil {
				e = &[3]int{}
				inter[h] = e
			}
			e[i]++
		}
		switch p.Kind {
		case "reply":
			bump(p.ReplyTo, 0)
		case "boost":
			bump(p.Author, 2)
		}
		if authored(p) {
			out.Authored++
			if len(p.Time) >= 13 {
				var h int
				fmt.Sscanf(p.Time[11:13], "%d", &h)
				if h >= 0 && h < 24 {
					out.Hours[h]++
				}
			}
			if t, ok := parseDate(p.Time); ok {
				out.Weekdays[int(t.Weekday())]++
			}
			for _, m := range p.Mentions {
				bump(m, 1)
			}
		}
		if writing(p) {
			proseN++
			texts = append(texts, p.Text)
			words += len(strings.Fields(p.Text))
			if len(p.Links) > 0 {
				linkN++
			}
			if len(p.Media) > 0 {
				mediaN++
			}
			if strings.Contains(p.Text, "?") {
				questN++
			}
			if hasEmoji(p.Text) {
				emojiN++
			}
			if shouting(p.Text) {
				shoutN++
			}
			if p.Kind == "reply" || p.Kind == "comment" {
				replyN++
			}
		}
	}

	for k, n := range kinds {
		out.Kinds = append(out.Kinds, models.Labelled{Label: kindLabel(k, n), N: n})
	}
	sort.Slice(out.Kinds, func(i, j int) bool {
		if out.Kinds[i].N != out.Kinds[j].N {
			return out.Kinds[i].N > out.Kinds[j].N
		}
		return out.Kinds[i].Label < out.Kinds[j].Label
	})
	var keys []string
	for k := range months {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 36 {
		keys = keys[len(keys)-36:]
	}
	for _, k := range keys {
		out.PerMonth = append(out.PerMonth, models.MonthCount{Month: k, N: months[k]})
	}
	best := 0
	for h, n := range out.Hours {
		if n > out.Hours[best] {
			best = h
		}
	}
	out.PeakHour = best
	out.Languages = languages(texts)
	out.Topics = topN(topics, 15)
	out.Domains = topN(domains, 10)
	for h, e := range inter {
		n := e[0]*3 + e[1] + e[2]*2
		var parts []string
		if e[0] > 0 {
			parts = append(parts, fmt.Sprintf("replied %d×", e[0]))
		}
		if e[1] > 0 {
			parts = append(parts, fmt.Sprintf("mentioned %d×", e[1]))
		}
		if e[2] > 0 {
			parts = append(parts, fmt.Sprintf("shared %d×", e[2]))
		}
		out.Interactions = append(out.Interactions, models.Labelled{Label: h, N: n, Note: strings.Join(parts, ", ")})
	}
	sort.Slice(out.Interactions, func(i, j int) bool {
		if out.Interactions[i].N != out.Interactions[j].N {
			return out.Interactions[i].N > out.Interactions[j].N
		}
		return out.Interactions[i].Label < out.Interactions[j].Label
	})
	if len(out.Interactions) > 12 {
		out.Interactions = out.Interactions[:12]
	}
	if proseN >= 5 {
		f := func(n int) float64 { return float64(int(float64(n)/float64(proseN)*100+0.5)) / 100 }
		out.Style = &models.WritingStyle{AvgWords: float64(int(float64(words)/float64(proseN)*10)) / 10, LinkShare: f(linkN), MediaShare: f(mediaN),
			QuestionShare: f(questN), EmojiShare: f(emojiN), ShoutShare: f(shoutN), ReplyShare: f(replyN)}
	} else if proseN > 0 {
		out.Notes = append(out.Notes, "Too little writing (fewer than five items) to describe a writing style.")
	}
	if out.Authored < 20 {
		out.Notes = append(out.Notes, "Fewer than 20 items the person wrote: the rhythm and the habits are only a hint.")
	}
	return out
}

func kindLabel(k string, n int) string {
	switch k {
	case "post":
		return plural1(n, "post", "posts")
	case "reply":
		return plural1(n, "reply", "replies")
	case "boost":
		return plural1(n, "boost", "boosts")
	case "comment":
		return plural1(n, "comment", "comments")
	case "commit":
		return plural1(n, "commit message", "commit messages")
	case "issue":
		return plural1(n, "issue", "issues")
	case "pull":
		return plural1(n, "pull request", "pull requests")
	case "repo":
		return plural1(n, "repository", "repositories")
	case "star":
		return plural1(n, "starred repository", "starred repositories")
	}
	return k
}

func plural1(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func topN(m map[string]int, n int) []models.Labelled {
	var out []models.Labelled
	for k, v := range m {
		out = append(out, models.Labelled{Label: k, N: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func hasEmoji(s string) bool {
	for _, r := range s {
		if (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) {
			return true
		}
	}
	return false
}

func shouting(s string) bool {
	var up, letters int
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				up++
			}
		}
	}
	return letters >= 12 && float64(up)/float64(letters) >= 0.6
}
