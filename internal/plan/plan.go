// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package plan describes, before anything runs, what a collection would gather: which collectors
// the question needs, which of the supplied details each would use, and where those details would
// be sent. It backs the "what will be collected" step that precedes every investigation, so the
// tool gathers the least that answers the question and the operator sees it first.
package plan

import "sort"

// Goal is the question an investigation answers. It picks the smallest sensible set of collectors.
type Goal struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Question string   `json:"question"`
	Modules  []string `json:"-"` // nil means every applicable collector
}

// Collectors that only ever work on something the operator supplied explicitly (a file, pasted links, a
// wallet, a place or period). They stay in every goal because they cannot reach for anything else.
var explicitOnly = []string{"uploads", "iddecode", "crypto", "context"}

// Goals in display order. The first is the default.
var Goals = []Goal{
	{ID: "identify", Label: "Identify who this is", Question: "Which public accounts and details belong to this person?",
		Modules: append([]string{"social", "profiles", "people", "images", "activity", "devtrace", "keys", "bskygraph", "footprint", "phone"}, explicitOnly...)},
	{ID: "verify", Label: "Check a claimed identity", Question: "Is this person who they say they are, and is anything against them?",
		Modules: append([]string{"people", "profiles", "social", "records", "sanctions", "mentions", "bizreg", "company", "devtrace", "keys"}, explicitOnly...)},
	{ID: "exposure", Label: "See what is exposed", Question: "What can anyone find about this account, address or number?",
		Modules: append([]string{"social", "profiles", "footprint", "phone", "breach", "images", "keys", "devtrace", "codesearch", "mentions", "activity"}, explicitOnly...)},
	{ID: "infrastructure", Label: "Investigate a domain, IP or company", Question: "Who runs this site, network or organisation, and what is exposed?",
		Modules: append([]string{"domain", "ip", "asn", "sitefiles", "docmeta", "lookalike", "webhistory", "company", "bizreg", "records", "syswrap", "dorks"}, explicitOnly...)},
	{ID: "everything", Label: "Everything available", Question: "Run every collector that applies to what I entered (slowest, sends the most).", Modules: nil},
}

// GoalByID returns the goal, or the default when the id is unknown or empty.
func GoalByID(id string) Goal {
	for _, g := range Goals {
		if g.ID == id {
			return g
		}
	}
	return Goals[0]
}

// Includes reports whether the goal wants the collector.
func (g Goal) Includes(name string) bool {
	if g.Modules == nil {
		return true
	}
	for _, m := range g.Modules {
		if m == name {
			return true
		}
	}
	return false
}

// Meta says what one collector does with the supplied details.
type Meta struct {
	Question string   // what it is for, in plain words
	Uses     []string // which supplied details it reads
	SendsTo  string   // who receives them; empty when nothing leaves this machine
	Weight   string   // light | medium | heavy: how many requests it makes
}

var meta = map[string]Meta{
	"social":     {"Which sites have an account with this username", []string{"username"}, "each site that is probed, one request per site", "heavy"},
	"profiles":   {"What the public profile of this username says", []string{"username"}, "GitHub, GitLab, Keybase, npm, Mastodon, Stack Overflow and similar public APIs", "medium"},
	"people":     {"Public profiles and records for this name", []string{"name", "username", "email", "location", "company"}, "LinkedIn public view, ORCID, Wikidata, OpenAlex, GitHub", "medium"},
	"images":     {"Public pictures of the subject", []string{"name", "username"}, "avatar hosts and web image search", "medium"},
	"activity":   {"When this username is active (time zone)", []string{"username"}, "GitHub, Hacker News, Reddit, Wikipedia, Bluesky, Mastodon, Lichess public activity", "medium"},
	"devtrace":   {"Developer traces of this username or address", []string{"username", "email"}, "GitHub, package registries and public keyservers", "medium"},
	"keys":       {"A published OpenPGP key", []string{"email", "username"}, "public keyservers and the address's own domain", "light"},
	"bskygraph":  {"Who a Bluesky account follows and is followed by", []string{"username"}, "the public Bluesky API", "light"},
	"footprint":  {"Whether this address or number has a public footprint", []string{"email", "phone"}, "DNS, Gravatar and web search", "light"},
	"phone":      {"Number validity, country and carrier type", []string{"phone"}, "", "light"},
	"breach":     {"Known breaches that include this address (names only)", []string{"email"}, "a public breach index", "light"},
	"codesearch": {"Public code that contains this address or domain", []string{"username", "email", "domain"}, "public code search", "medium"},
	"mentions":   {"News and press that name the person", []string{"name"}, "public news search", "medium"},
	"records":    {"Official filings and scan history", []string{"name", "domain", "ip"}, "SEC EDGAR and urlscan.io", "light"},
	"sanctions":  {"Whether the name appears on sanctions lists", []string{"name", "company"}, "downloads the public OFAC and UN lists and compares them here", "light"},
	"bizreg":     {"Official company registers", []string{"company"}, "open company registers (France, Norway)", "light"},
	"company":    {"An organisation's sites and public pages", []string{"company"}, "Wikipedia, Wikidata and GitHub", "light"},
	"domain":     {"Who runs this domain and what is public", []string{"domain"}, "DNS, RDAP, certificate logs, the site itself and web archives", "heavy"},
	"ip":         {"Network owner, location and exposure of this IP", []string{"ip"}, "RDAP, geolocation and passive scan data", "medium"},
	"asn":        {"Owner and peers of this network", []string{"asn"}, "RDAP and routing data", "light"},
	"sitefiles":  {"People a site publishes about itself", []string{"domain"}, "the site itself", "light"},
	"docmeta":    {"Author and software hidden in published documents", []string{"domain"}, "the site itself", "medium"},
	"lookalike":  {"Look-alike domains that exist", []string{"domain"}, "DNS", "heavy"},
	"webhistory": {"Old versions and tracking IDs of a site", []string{"domain"}, "web archives", "medium"},
	"dorks":      {"What search engines show for these details", []string{"name", "email", "username", "phone", "domain"}, "web search engines", "medium"},
	"syswrap":    {"Output of external tools you have installed", []string{"domain", "username"}, "whatever the installed tools contact", "medium"},
	"uploads":    {"Metadata and text inside files you added", []string{"files"}, "", "light"},
	"iddecode":   {"When a pasted link or ID was created", []string{"notes"}, "", "light"},
	"crypto":     {"Activity of a wallet address", []string{"wallet"}, "public blockchain explorers and ENS", "light"},
	"context":    {"Nearby places and news for a place and period", []string{"coords", "period"}, "map tiles, geo search and news archives", "medium"},
}

func init() {
	for name, what := range map[string]string{
		"postsmastodon": "Mastodon", "postsbluesky": "Bluesky", "postsgithub": "GitHub", "postsreddit": "Reddit",
	} {
		meta[name] = Meta{Question: "Copy the public " + what + " posts of the username you typed", Uses: []string{"username"},
			SendsTo: map[string]string{
				"Mastodon": "the Mastodon servers' public API (no sign-in)", "Bluesky": "the public Bluesky API (no sign-in)",
				"GitHub": "the public GitHub API (no sign-in)", "Reddit": "Reddit's public JSON pages (no sign-in; often refused)"}[what],
			Weight: "light"}
	}
}

// IsOptional reports whether a collector is opt-in: it copies content, so it is never proposed and runs only when ticked.
func IsOptional(name string) bool {
	switch name {
	case "postsmastodon", "postsbluesky", "postsgithub", "postsreddit":
		return true
	}
	return false
}

// Of returns what is known about a collector (a generic entry for one added later).
func Of(name string) Meta {
	if m, ok := meta[name]; ok {
		return m
	}
	return Meta{Question: "Runs on the details you entered", SendsTo: "the services it queries", Weight: "medium"}
}

// Names lists the collectors that have a description here, sorted.
func Names() []string {
	out := make([]string, 0, len(meta))
	for k := range meta {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
