// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package insights derives conclusions from a finished report with fixed, documented rules: a brief, next
// actions, a persona, an authenticity estimate, associates, hidden links, a life timeline with anomalies,
// movement and the reasoning behind each fact. No model is involved and nothing is fetched: the same
// report always gives the same insights.
package insights

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// parseDate reads YYYY-MM-DD (possibly followed by a time) and the EXIF form YYYY:MM:DD HH:MM:SS.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 10 {
		return time.Time{}, false
	}
	d := strings.ReplaceAll(s[:10], ":", "-")
	t, err := time.Parse("2006-01-02", d)
	if err != nil || t.Year() < 1970 || t.After(time.Now().AddDate(1, 0, 0)) {
		return time.Time{}, false
	}
	return t, true
}

func dayString(t time.Time) string { return t.Format("2006-01-02") }

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func containsFold(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

var wordRe = regexp.MustCompile(`[\p{L}\p{N}']+`)

func words(s string) []string {
	var out []string
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		out = append(out, w)
	}
	return out
}

// stem reduces a handle to its comparable core: lower case, letters only (digits, dots, dashes and underscores go).
func stem(h string) string {
	h = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "@"))
	var b strings.Builder
	for _, r := range h {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
