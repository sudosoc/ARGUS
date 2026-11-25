// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package lookalike

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Candidate is a look-alike name and the trick that produced it.
type Candidate struct {
	Domain    string // ASCII form (punycode for IDN)
	Display   string // Unicode form shown to people
	Technique string
}

var qwerty = map[rune]string{
	'q': "wa", 'w': "qeas", 'e': "wrsd", 'r': "etdf", 't': "ryfg", 'y': "tugh", 'u': "yihj", 'i': "uojk", 'o': "ipkl", 'p': "ol",
	'a': "qwsz", 's': "qweadzx", 'd': "wersfxc", 'f': "ertdgcv", 'g': "rtyfhvb", 'h': "tyugjbn", 'j': "yuihknm", 'k': "uiojlm", 'l': "iopk",
	'z': "asx", 'x': "zsdc", 'c': "xdfv", 'v': "cfgb", 'b': "vghn", 'n': "bhjm", 'm': "njk",
}

// confusables maps a letter to look-alike strings (Latin digits and
// multi-letter shapes first, then the Cyrillic/Greek letters that render
// identically in most fonts).
var confusables = map[string][]string{
	"a": {"4", "а", "ɑ"}, "b": {"d", "6", "Ь"}, "c": {"е" + "", "с"}, "d": {"b", "cl", "ԁ"}, "e": {"3", "е"}, "g": {"q", "9"},
	"h": {"n", "һ"}, "i": {"1", "l", "і", "í"}, "j": {"ј"}, "k": {"lc", "к"}, "l": {"1", "i", "I", "ӏ"}, "m": {"rn", "nn", "м"},
	"n": {"m", "r", "п"}, "o": {"0", "о", "ο"}, "p": {"q", "р"}, "q": {"p", "g"}, "r": {"n", "г"}, "s": {"5", "ѕ"}, "t": {"7", "т"},
	"u": {"v", "υ"}, "v": {"u", "w", "ѵ"}, "w": {"vv", "ѡ"}, "x": {"х"}, "y": {"у", "ү"}, "z": {"2"},
}

var tldSwaps = []string{"com", "net", "org", "co", "io", "info", "biz", "app", "xyz", "online", "site", "top", "cc", "us", "co.uk", "de", "fr", "ru", "cn", "me", "dev", "cloud", "shop", "support"}

var keywords = []string{"login", "secure", "support", "account", "mail", "portal", "online", "service", "official", "app", "my", "web"}

var secondLevel = map[string]bool{"co": true, "com": true, "org": true, "net": true, "gov": true, "ac": true, "edu": true}

// Split separates a registrable domain into its label and public suffix
// (only the common two-part suffixes are recognised).
func Split(domain string) (sld, tld string, ok bool) {
	parts := strings.Split(strings.ToLower(strings.Trim(domain, ".")), ".")
	if len(parts) < 2 {
		return "", "", false
	}
	n := 1
	if len(parts) >= 3 && len(parts[len(parts)-1]) == 2 && secondLevel[parts[len(parts)-2]] {
		n = 2
	}
	if len(parts)-n < 1 {
		return "", "", false
	}
	// A registrable name is the label before the suffix; deeper labels are subdomains.
	return parts[len(parts)-n-1], strings.Join(parts[len(parts)-n:], "."), true
}

// Generate returns up to max distinct look-alikes of domain, most plausible
// techniques first. The original name is never included.
func Generate(domain string, max int) []Candidate {
	sld, tld, ok := Split(domain)
	if !ok || utf8.RuneCountInString(sld) < 2 {
		return nil
	}
	orig := sld + "." + tld
	seen := map[string]bool{orig: true}
	var out []Candidate
	add := func(label, suffix, tech string) {
		label = strings.Trim(label, "-")
		if label == "" || len(label) > 63 {
			return
		}
		display := label + "." + suffix
		ascii, err := toASCII(display)
		if err != nil || seen[ascii] || seen[display] {
			return
		}
		seen[ascii], seen[display] = true, true
		out = append(out, Candidate{Domain: ascii, Display: display, Technique: tech})
	}
	rs := []rune(sld)

	for i := range rs { // omission
		add(string(rs[:i])+string(rs[i+1:]), tld, "omission")
	}
	for i := 0; i+1 < len(rs); i++ { // transposition
		if rs[i] != rs[i+1] {
			t := append([]rune(nil), rs...)
			t[i], t[i+1] = t[i+1], t[i]
			add(string(t), tld, "transposition")
		}
	}
	for i, r := range rs { // repetition
		add(string(rs[:i+1])+string(r)+string(rs[i+1:]), tld, "repetition")
	}
	for i, r := range rs { // keyboard-neighbour replacement and insertion
		for _, n := range qwerty[r] {
			add(string(rs[:i])+string(n)+string(rs[i+1:]), tld, "keyboard typo")
		}
	}
	for i, r := range rs {
		for _, n := range qwerty[r] {
			add(string(rs[:i])+string(n)+string(rs[i:]), tld, "keyboard typo")
		}
	}
	for i := 1; i < len(rs); i++ { // hyphenation / dot
		add(string(rs[:i])+"-"+string(rs[i:]), tld, "hyphenation")
	}
	for i, r := range rs { // look-alike characters (the IDN homograph attack)
		for _, c := range confusables[string(r)] {
			tech := "look-alike character"
			if !isASCII(c) {
				tech = "IDN homograph"
			}
			add(string(rs[:i])+c+string(rs[i+1:]), tld, tech)
		}
	}
	for i, r := range rs { // bit-squatting: one flipped bit still a valid hostname character
		if r > 127 {
			continue
		}
		for b := 0; b < 8; b++ {
			f := r ^ (1 << b)
			if (f >= 'a' && f <= 'z') || (f >= '0' && f <= '9') || f == '-' {
				add(string(rs[:i])+string(f)+string(rs[i+1:]), tld, "bit-squatting")
			}
		}
	}
	for _, v := range []rune("aeiou") { // vowel swap
		for i, r := range rs {
			if strings.ContainsRune("aeiou", r) && r != v {
				add(string(rs[:i])+string(v)+string(rs[i+1:]), tld, "vowel swap")
			}
		}
	}
	for _, t := range tldSwaps { // other TLDs
		if t != tld {
			add(sld, t, "other TLD")
		}
	}
	add("www"+sld, tld, "missing dot after www")
	for _, k := range keywords { // phishing-style keywords
		add(sld+"-"+k, tld, "added keyword")
		add(k+"-"+sld, tld, "added keyword")
		add(sld+k, tld, "added keyword")
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// toASCII converts a Unicode domain to its punycode (xn--) form.
func toASCII(d string) (string, error) {
	labels := strings.Split(d, ".")
	for i, l := range labels {
		if isASCII(l) {
			continue
		}
		p, err := punycode(l)
		if err != nil {
			return "", err
		}
		labels[i] = "xn--" + p
	}
	return strings.Join(labels, "."), nil
}

// punycode implements RFC 3492 encoding.
func punycode(s string) (string, error) {
	const (
		base, tmin, tmax, skew, damp = 36, 1, 26, 38, 700
		initialBias, initialN        = 72, 128
	)
	var out []byte
	runes := []rune(strings.ToLower(s))
	for _, r := range runes {
		if r < 128 {
			out = append(out, byte(r))
		}
	}
	b := len(out)
	h := b
	if b > 0 {
		out = append(out, '-')
	}
	adapt := func(delta, numPoints int, first bool) int {
		if first {
			delta /= damp
		} else {
			delta /= 2
		}
		delta += delta / numPoints
		k := 0
		for delta > ((base-tmin)*tmax)/2 {
			delta /= base - tmin
			k += base
		}
		return k + (base-tmin+1)*delta/(delta+skew)
	}
	digit := func(d int) byte {
		if d < 26 {
			return byte('a' + d)
		}
		return byte('0' + d - 26)
	}
	n, delta, bias := initialN, 0, initialBias
	for h < len(runes) {
		m := int(^uint(0) >> 1)
		for _, r := range runes {
			if int(r) >= n && int(r) < m {
				m = int(r)
			}
		}
		delta += (m - n) * (h + 1)
		n = m
		for _, r := range runes {
			if int(r) < n {
				delta++
			}
			if int(r) == n {
				q := delta
				for k := base; ; k += base {
					t := k - bias
					if t < tmin {
						t = tmin
					} else if t > tmax {
						t = tmax
					}
					if q < t {
						break
					}
					out = append(out, digit(t+(q-t)%(base-t)))
					q = (q - t) / (base - t)
				}
				out = append(out, digit(q))
				bias = adapt(delta, h+1, h == b)
				delta = 0
				h++
			}
		}
		delta++
		n++
	}
	return string(out), nil
}

// Techniques summarises how many candidates each technique produced.
func Techniques(c []Candidate) map[string]int {
	m := map[string]int{}
	for _, x := range c {
		m[x.Technique]++
	}
	return m
}

// SortedTechniques returns the technique names in a stable order.
func SortedTechniques(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
