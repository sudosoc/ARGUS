// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/sudosoc/argus/internal/models"
)

// isCreation reports whether a timeline event says an account or a record came into being.
func isCreation(event string) bool {
	e := strings.ToLower(event)
	return strings.Contains(e, "created") || strings.Contains(e, "joined") || strings.Contains(e, "member since") || strings.Contains(e, "registered")
}

// ownRecords returns the person's records (profiles) found by the engine.
func ownRecords(c *ctx) []*models.Record {
	if c.primary == nil {
		return nil
	}
	urls := map[string]bool{}
	for _, r := range c.primary.Records {
		urls[r.URL] = true
	}
	var out []*models.Record
	for i := range c.hits {
		r := c.hits[i].Record
		if r != nil && urls[r.URL] {
			out = append(out, r)
		}
	}
	return out
}

func persona(c *ctx) *models.Persona {
	if c.primary == nil && len(c.hits) == 0 {
		return nil
	}
	p := &models.Persona{}
	recs := ownRecords(c)

	// ---- languages from what the person wrote about themself
	var texts []string
	if c.primary != nil {
		texts = append(texts, c.primary.Bio)
		for _, f := range c.primary.Headlines {
			texts = append(texts, f.Value)
		}
		for _, f := range c.primary.Interests {
			texts = append(texts, f.Value)
		}
	}
	for _, r := range recs {
		texts = append(texts, r.Bio, r.Headline)
	}
	p.Languages = languages(texts)

	// ---- expertise from developer traces, interests and headlines
	p.Expertise = expertise(c)

	// ---- daily rhythm
	p.Schedule = schedule(c.rep.Geo)

	// ---- platforms, first and latest trace
	plats := map[string]bool{}
	if c.primary != nil {
		for _, r := range c.primary.Records {
			if r.Platform != "" {
				plats[r.Platform] = true
			}
		}
	}
	for i := range c.hits {
		f := c.hits[i]
		if f.Module == "social" && f.Status == models.StatusFound {
			plats[strings.TrimSuffix(f.Source, " profile")] = true
		}
	}
	for k := range plats {
		p.Platforms = append(p.Platforms, k)
	}
	sort.Strings(p.Platforms)
	p.Accounts = len(p.Platforms)
	var first, last string
	for _, ev := range c.rep.Timeline {
		if _, ok := parseDate(ev.Date); !ok {
			continue
		}
		if isCreation(ev.Event) && (first == "" || ev.Date < first) {
			first, p.FirstSource = ev.Date, ev.Source
		}
		if last == "" || ev.Date > last {
			last = ev.Date
		}
	}
	if first != "" {
		p.FirstTrace = first[:10]
		if t0, ok := parseDate(first); ok {
			if t1, ok2 := parseDate(last); ok2 && t1.After(t0) {
				p.YearsOnline = float64(int(t1.Sub(t0).Hours()/24/365.25*10)) / 10
			}
		}
	}
	if last != "" {
		p.LatestTrace = last[:10]
	}

	p.Habits, p.Exposure = habits(c, p)
	if len(p.Languages) == 0 && len(p.Expertise) == 0 && p.Schedule == nil && len(p.Habits) == 0 && len(p.Exposure) == 0 && p.FirstTrace == "" {
		return nil
	}
	return p
}

// ------------------------------------------------------------------ languages

var stopwords = map[string][]string{
	"English":    {"the", "and", "of", "to", "in", "is", "for", "with", "on", "at", "my", "an", "are", "i", "from"},
	"French":     {"le", "la", "les", "des", "et", "est", "une", "dans", "pour", "avec", "du", "sur", "je", "au", "qui"},
	"Spanish":    {"el", "los", "las", "una", "y", "es", "en", "por", "con", "del", "para", "soy", "que", "mi", "un"},
	"German":     {"der", "die", "das", "und", "ist", "ein", "eine", "mit", "für", "ich", "von", "auf", "nicht", "zu", "den"},
	"Portuguese": {"o", "os", "as", "uma", "e", "em", "por", "com", "do", "da", "para", "sou", "que", "meu", "um"},
	"Italian":    {"il", "lo", "gli", "una", "e", "di", "che", "per", "con", "del", "sono", "non", "un", "della", "nel"},
	"Turkish":    {"bir", "ve", "bu", "için", "ile", "ben", "de", "da", "çok", "gibi", "daha", "olan", "ama", "en", "mi"},
	"Dutch":      {"de", "het", "een", "en", "van", "ik", "is", "op", "te", "met", "voor", "niet", "dat", "zijn", "aan"},
}

func scriptOf(r rune) string {
	switch {
	case unicode.Is(unicode.Arabic, r):
		return "Arabic"
	case unicode.Is(unicode.Cyrillic, r):
		return "Russian / Cyrillic"
	case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
		return "Chinese / Japanese"
	case unicode.Is(unicode.Hangul, r):
		return "Korean"
	case unicode.Is(unicode.Hebrew, r):
		return "Hebrew"
	case unicode.Is(unicode.Devanagari, r):
		return "Hindi / Devanagari"
	case unicode.Is(unicode.Thai, r):
		return "Thai"
	case unicode.Is(unicode.Greek, r):
		return "Greek"
	case unicode.Is(unicode.Latin, r):
		return "Latin"
	}
	return ""
}

// languages guesses the languages a person writes in from the text they published about themself.
// Scripts are exact; among Latin-script languages a small stop-word vote decides, and an undecided
// text is reported as "Latin script" rather than guessed.
func languages(texts []string) []models.Labelled {
	counts := map[string]int{}
	total := 0
	var latin strings.Builder
	for _, t := range texts {
		for _, r := range t {
			if !unicode.IsLetter(r) {
				continue
			}
			s := scriptOf(r)
			if s == "" {
				continue
			}
			counts[s]++
			total++
		}
		latin.WriteString(" " + t)
	}
	if total < 25 {
		return nil
	}
	if counts["Latin"] > 0 {
		lang, note := latinLanguage(latin.String())
		counts[lang] += counts["Latin"]
		delete(counts, "Latin")
		_ = note
	}
	var out []models.Labelled
	for k, n := range counts {
		share := float64(n) / float64(total)
		if share < 0.1 {
			continue
		}
		out = append(out, models.Labelled{Label: k, Share: float64(int(share*100)) / 100, N: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N })
	return out
}

func latinLanguage(text string) (string, string) {
	ws := words(text)
	hits := map[string]int{}
	for lang, list := range stopwords {
		set := map[string]bool{}
		for _, w := range list {
			set[w] = true
		}
		for _, w := range ws {
			if set[w] {
				hits[lang]++
			}
		}
	}
	best, bn, second := "", 0, 0
	for l, n := range hits {
		if n > bn {
			best, second, bn = l, bn, n
		} else if n > second {
			second = n
		}
	}
	if bn >= 2 && float64(bn) >= 1.3*float64(second) {
		return best, ""
	}
	return "Latin script", "language not determined"
}

// ------------------------------------------------------------------ expertise

func parseCounted(s string) []models.Labelled {
	var out []models.Labelled
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n := 1
		if i := strings.LastIndex(part, " ("); i > 0 && strings.HasSuffix(part, ")") {
			if v, err := strconv.Atoi(part[i+2 : len(part)-1]); err == nil {
				n = v
				part = part[:i]
			}
		}
		out = append(out, models.Labelled{Label: part, N: n})
	}
	return out
}

func expertise(c *ctx) []models.Labelled {
	agg := map[string]*models.Labelled{}
	add := func(l models.Labelled, note string) {
		k := strings.ToLower(l.Label)
		if k == "" {
			return
		}
		if e, ok := agg[k]; ok {
			e.N += l.N
			if note != "" && !strings.Contains(e.Note, note) {
				e.Note += ", " + note
			}
			return
		}
		l.Note = note
		agg[k] = &l
	}
	for i := range c.hits {
		f := c.hits[i]
		if f.Module != "devtrace" {
			continue
		}
		for _, l := range parseCounted(f.Details["main languages"]) {
			add(l, "code")
		}
		for _, l := range parseCounted(f.Details["topics"]) {
			add(l, "repository topics")
		}
	}
	if c.primary != nil {
		for _, f := range c.primary.Interests {
			add(models.Labelled{Label: f.Value, N: len(f.Sources) + 1}, "stated interest")
		}
		for _, f := range c.primary.Headlines {
			add(models.Labelled{Label: f.Value, N: len(f.Sources) + 2}, "headline")
		}
	}
	out := make([]models.Labelled, 0, len(agg))
	for _, v := range agg {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Label < out[j].Label
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// ------------------------------------------------------------------- schedule

func schedule(g *models.GeoSummary) *models.Schedule {
	if g == nil || g.TZ == nil || g.TZ.Events < 20 {
		return nil
	}
	tz := g.TZ
	off := int(tz.Offset)
	var local [24]int
	for h := 0; h < 24; h++ {
		local[((h+off)%24+24)%24] += tz.Hours[h]
	}
	// quietest 6-hour window (circular)
	best, bestN := 0, 1<<30
	for s := 0; s < 24; s++ {
		n := 0
		for k := 0; k < 6; k++ {
			n += local[(s+k)%24]
		}
		if n < bestN {
			best, bestN = s, n
		}
	}
	peak, peakN := 0, -1
	for h, n := range local {
		if n > peakN {
			peak, peakN = h, n
		}
	}
	night := 0
	for h := 0; h < 24; h++ {
		if h >= 22 || h < 5 {
			night += local[h]
		}
	}
	weekend, all := 0, 0
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			all += tz.Grid[d][h]
			if d >= 5 {
				weekend += tz.Grid[d][h]
			}
		}
	}
	s := &models.Schedule{UTCOffset: tz.Offset, Zone: tz.Label, SleepFrom: best, SleepTo: (best + 6) % 24, PeakHour: peak, Events: tz.Events}
	if tz.Events > 0 {
		s.NightShare = float64(int(float64(night)/float64(tz.Events)*100)) / 100
	}
	if all > 0 {
		s.WeekendShare = float64(int(float64(weekend)/float64(all)*100)) / 100
	}
	var pat []string
	switch {
	case s.NightShare >= 0.30:
		pat = append(pat, "a night owl (a third of activity falls between 22:00 and 05:00 local)")
	case peak >= 9 && peak <= 17:
		pat = append(pat, "mostly active during working hours")
	case peak >= 18 && peak <= 23:
		pat = append(pat, "mostly active in the evening")
	}
	switch {
	case all > 0 && s.WeekendShare >= 0.40:
		pat = append(pat, "more active at weekends than on weekdays")
	case all > 0 && s.WeekendShare <= 0.12:
		pat = append(pat, "almost silent at weekends")
	}
	if len(pat) == 0 {
		pat = append(pat, "no strong daily pattern")
	}
	s.Pattern = strings.Join(pat, "; ")
	return s
}

// --------------------------------------------------------------------- habits

// habits returns behaviours seen and what they give away.
func habits(c *ctx, p *models.Persona) (seen, exposure []string) {
	handles := map[string]map[string]bool{}
	for i := range c.hits {
		f := c.hits[i]
		if f.Module != "social" || f.Status != models.StatusFound {
			continue
		}
		h := strings.ToLower(f.Details["username"])
		if h == "" {
			continue
		}
		if handles[h] == nil {
			handles[h] = map[string]bool{}
		}
		handles[h][f.Source] = true
	}
	topH, topN := "", 0
	for h, set := range handles {
		if len(set) > topN {
			topH, topN = h, len(set)
		}
	}
	if topN >= 3 {
		seen = append(seen, fmt.Sprintf("Uses the handle “%s” on %d platforms.", topH, topN))
		exposure = append(exposure, fmt.Sprintf("One search for “%s” finds %d accounts: the handle is reused almost everywhere.", topH, topN))
	}
	if c.primary != nil {
		for _, e := range c.primary.Emails {
			local := strings.ToLower(strings.SplitN(e.Value, "@", 2)[0])
			for _, u := range c.primary.Usernames {
				h := strings.ToLower(u.Value)
				if i := strings.Index(h, ":"); i >= 0 {
					h = h[i+1:]
				}
				if local != "" && local == h {
					exposure = append(exposure, fmt.Sprintf("The e-mail address %s uses the same name as the handle “%s”: anyone with one finds the other.", e.Value, h))
				}
			}
		}
		for _, n := range c.primary.Names {
			if n.Confirmed {
				seen = append(seen, fmt.Sprintf("Publishes the real name “%s” on %d platforms.", n.Value, len(n.Sources)))
				break
			}
		}
	}
	shared := 0
	gps, serial := 0, 0
	for _, ph := range c.rep.Photos {
		if ph.Group > 0 && len(ph.Sources) >= 2 {
			shared++
		}
		if ph.GPS != "" {
			gps++
		}
		if ph.Meta["SerialNumber"] != "" || ph.Meta["BodySerialNumber"] != "" {
			serial++
		}
	}
	if shared > 0 {
		seen = append(seen, fmt.Sprintf("Uses the same picture on several accounts (%s).", plural(shared, "picture", "pictures")))
		exposure = append(exposure, "The same profile picture links the accounts: a reverse image search joins them.")
	}
	if gps > 0 {
		exposure = append(exposure, fmt.Sprintf("%s still carry GPS coordinates.", plural(gps, "published picture", "published pictures")))
	}
	if serial > 0 {
		exposure = append(exposure, "Published pictures carry a camera serial number, which links them to one device.")
	}
	for i := range c.hits {
		f := c.hits[i]
		switch {
		case f.Module == "devtrace" && strings.Contains(f.Details["authors in public commits"], "@"):
			exposure = append(exposure, "Personal e-mail addresses appear in public code commits.")
		case f.Module == "breach" && f.Category == "breach":
			exposure = append(exposure, "An e-mail address appears in known breaches ("+firstNonEmpty(f.Details["breach names"], f.Details["breaches"], f.Source)+").")
		case strings.Contains(strings.ToLower(f.Source+f.Category), "stealer"):
			exposure = append(exposure, "An address appears in infostealer logs: a device of the person was infected.")
		}
	}
	return uniqStrings(seen), uniqStrings(exposure)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
