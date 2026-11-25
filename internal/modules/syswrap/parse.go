// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package syswrap

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sudosoc/argus/internal/models"
)

// maxItems is a safety valve against a runaway tool, not a result limit: the complete
// output is always stored separately (see syswrap.run).
const maxItems = 1 << 20

func finding(source, category, url string, conf float64, d map[string]string) models.Finding {
	return models.Finding{Source: source, Category: category, URL: url,
		Status: models.StatusFound, Confidence: conf, Details: d}
}

// parseWhois pulls the intelligence-bearing "Key: value" lines out of whois
// output and merges repeated keys (name servers) into comma lists.
func parseWhois(in Inputs, _ []string, out string) []models.Finding {
	want := map[string]string{
		"registrar": "registrar", "registrant name": "registrant_name",
		"registrant organization": "registrant_org", "registrant email": "registrant_email",
		"registrant country": "registrant_country", "creation date": "created",
		"registry expiry date": "expires", "updated date": "updated",
		"name server": "name_servers", "registrar abuse contact email": "abuse_email",
		"admin email": "admin_email", "tech email": "tech_email", "dnssec": "dnssec",
	}
	d := map[string]string{}
	seen := map[string]map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, known := want[strings.ToLower(strings.TrimSpace(k))]
		v = strings.TrimSpace(v)
		if !known || v == "" || strings.Contains(strings.ToLower(v), "redacted") {
			continue
		}
		if seen[key] == nil {
			seen[key] = map[string]bool{}
		}
		lv := strings.ToLower(v)
		if seen[key][lv] {
			continue
		}
		seen[key][lv] = true
		if old, exists := d[key]; exists {
			d[key] = old + ", " + v
		} else {
			d[key] = v
		}
	}
	if len(d) == 0 {
		return nil
	}
	d["domain"] = in.Domain
	return []models.Finding{finding("whois "+in.Domain, "domain", "", 0.9, d)}
}

// parseDig turns `dig +short <type> <domain>` into one finding per record type.
func parseDig(in Inputs, args []string, out string) []models.Finding {
	var recs []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, ";") {
			recs = append(recs, l)
		}
	}
	if len(recs) == 0 {
		return nil
	}
	typ := "?"
	for _, a := range args {
		switch a {
		case "MX", "TXT", "NS", "A", "AAAA":
			typ = a
		}
	}
	if len(recs) > maxItems {
		recs = recs[:maxItems]
	}
	return []models.Finding{finding("dig "+typ+" "+in.Domain, "dns", "", 0.95,
		map[string]string{"type": typ, "records": strings.Join(recs, " | ")})}
}

var harvestHeadRe = regexp.MustCompile(`(?i)^\[\*\]\s+(Emails|Hosts|IPs|URLs|ASNS|Interesting Urls|Linkedin Links|Twitter Handles?)\b.*found`)

// parseHarvester reads theHarvester's console report: sections introduced by
// "[*] <Kind> found: N", a dashed rule, then one item per line until a blank
// line or the next "[*]" header.
func parseHarvester(in Inputs, _ []string, out string) []models.Finding {
	var res []models.Finding
	kind := ""
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if m := harvestHeadRe.FindStringSubmatch(line); m != nil {
			kind = strings.ToLower(m[1])
			continue
		}
		if line == "" || strings.HasPrefix(line, "[*]") {
			kind = ""
			continue
		}
		if kind == "" || strings.Trim(line, "-") == "" || len(res) >= maxItems {
			continue
		}
		item, ip, _ := strings.Cut(line, ":") // hosts print as "host:1.2.3.4"
		d := map[string]string{"type": kind, "domain": in.Domain}
		if ip != "" && kind == "hosts" {
			d["ip"] = ip
		}
		f := finding("theHarvester", kind, "", 0.7, d)
		if kind == "emails" {
			d["email"] = item
		} else {
			f.URL = item
		}
		res = append(res, f)
	}
	return res
}

var sherlockRe = regexp.MustCompile(`^\[\+\]\s+(.+?):\s+(https?://\S+)\s*$`)

// parseSherlock reads "[+] Site: URL" lines from `sherlock --print-found`.
func parseSherlock(in Inputs, _ []string, out string) []models.Finding {
	var res []models.Finding
	for _, l := range strings.Split(out, "\n") {
		if m := sherlockRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil && len(res) < maxItems {
			res = append(res, finding(m[1], "social", m[2], 0.7,
				map[string]string{"username": in.Username, "via": "sherlock"}))
		}
	}
	return res
}

// parseSubfinder reads one subdomain per line.
func parseSubfinder(in Inputs, _ []string, out string) []models.Finding {
	var hosts []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" && strings.HasSuffix(l, in.Domain) && len(hosts) < maxItems {
			hosts = append(hosts, l)
		}
	}
	if len(hosts) == 0 {
		return nil
	}
	return []models.Finding{finding("subfinder "+in.Domain, "subdomain", "", 0.8,
		map[string]string{"count": strconv.Itoa(len(hosts)), "subdomains": strings.Join(hosts, ", ")})}
}

// parseWhatweb keeps WhatWeb's fingerprint of the site, every line of it.
func parseWhatweb(in Inputs, _ []string, out string) []models.Finding {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	d := map[string]string{"fingerprint": out}
	if lines := strings.Split(out, "\n"); len(lines) > 1 {
		d["fingerprint"] = strings.Join(lines, "\n")
	}
	return []models.Finding{finding("whatweb "+in.Domain, "tech", "", 0.7, d)}
}

// parseFoundLines reads "[+] Site: URL" lines (Sherlock/Maigret style).
func parseFoundLines(via string) func(Inputs, []string, string) []models.Finding {
	return func(in Inputs, _ []string, out string) []models.Finding {
		var res []models.Finding
		for _, l := range strings.Split(out, "\n") {
			if m := sherlockRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil && len(res) < maxItems {
				res = append(res, finding(m[1], "social", m[2], 0.7,
					map[string]string{"username": in.Username, "via": via}))
			}
		}
		return res
	}
}

var holeheLine = regexp.MustCompile(`^\[([+\-x!?])\]\s+(\S+)\s*(.*)$`)
var holeheExtra = regexp.MustCompile(`(?i)^(Email Recovery|Phone Number|Others?)\s*:\s*(.+)$`)

// parseHolehe keeps every outcome holehe printed: the sites where the address
// has an account (with the recovery hints holehe prints), the sites where it
// has none, and the sites that rate-limited or failed (an unknown result, listed
// as a coverage gap rather than dropped).
func parseHolehe(in Inputs, _ []string, out string) []models.Finding {
	var used, unused, limited []string
	notes := map[string][]string{}
	var site string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if m := holeheLine.FindStringSubmatch(l); m != nil {
			site = m[2]
			switch m[1] {
			case "+":
				used = append(used, site)
			case "-":
				unused = append(unused, site)
			default: // x, !, ?
				limited = append(limited, site)
			}
			continue
		}
		if m := holeheExtra.FindStringSubmatch(l); m != nil && site != "" {
			notes[site] = append(notes[site], m[1]+": "+strings.TrimSpace(m[2]))
		}
	}
	var res []models.Finding
	if len(used) > 0 {
		d := map[string]string{"count": strconv.Itoa(len(used)), "sites": strings.Join(used, ", "),
			"checked": strconv.Itoa(len(used) + len(unused) + len(limited)), "not registered on": strings.Join(unused, ", ")}
		if len(limited) > 0 {
			d["rate-limited or failed (result unknown)"] = strings.Join(limited, ", ")
		}
		var hints []string
		for _, s := range used {
			if n := notes[s]; len(n) > 0 {
				hints = append(hints, s+" → "+strings.Join(n, "; "))
			}
		}
		if len(hints) > 0 {
			d["recovery hints"] = strings.Join(hints, "\n")
		}
		res = append(res, finding("holehe: accounts registered with "+in.Email, "email-account", "", 0.75, d))
	} else if len(unused) > 0 {
		res = append(res, models.Finding{Source: "holehe: accounts registered with " + in.Email, Category: "email-account", Status: models.StatusNotFound,
			Details: map[string]string{"checked": strconv.Itoa(len(unused) + len(limited)), "not registered on": strings.Join(unused, ", ")}})
	}
	for _, s := range limited {
		res = append(res, models.Finding{Source: "holehe: " + s, Category: "email-account", Status: models.StatusError,
			Details: map[string]string{"error": "rate-limited or failed: whether " + in.Email + " has an account here is unknown"}})
	}
	return res
}

// parseRawSummary keeps the first lines of a tool's report verbatim.
func parseRawSummary(name, category string) func(Inputs, []string, string) []models.Finding {
	return func(in Inputs, _ []string, out string) []models.Finding {
		out = strings.TrimSpace(out)
		if out == "" {
			return nil
		}
		return []models.Finding{finding(name, category, "", 0.6, map[string]string{"output": out})}
	}
}
