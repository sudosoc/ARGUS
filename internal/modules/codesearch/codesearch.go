// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package codesearch searches public source code for an e-mail address or a
// domain. People leave their address in commit-less places no other source
// looks at: package metadata, config files, docs, test fixtures, scripts.
// It uses grep.app's public search endpoint (no account); that endpoint is
// unofficial, so a failure is reported as a coverage gap, not as "nothing".
package codesearch

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/intel"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

var endpoint = "https://grep.app/api/search"

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "codesearch" }
func (s *Scanner) Description() string {
	return "public source code that contains the e-mail address (package metadata, configs, docs, scripts)"
}
func (s *Scanner) CanRun(t models.Target) bool {
	e := strings.TrimSpace(t.Email)
	if i := strings.LastIndex(e, "@"); i > 0 {
		return !intel.IsDisposable(e[i+1:])
	}
	return false
}

type response struct {
	Hits struct {
		Total any `json:"total"`
		Hits  []struct {
			Repo struct {
				Raw string `json:"raw"`
			} `json:"repo"`
			Path struct {
				Raw string `json:"raw"`
			} `json:"path"`
			Content struct {
				Snippet string `json:"snippet"`
			} `json:"content"`
		} `json:"hits"`
	} `json:"hits"`
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// Hit is one file that mentions the query.
type Hit struct{ Repo, Path, Snippet string }

func parse(r response) (hits []Hit, total int) {
	switch v := r.Hits.Total.(type) {
	case float64:
		total = int(v)
	case map[string]any:
		if f, ok := v["value"].(float64); ok {
			total = int(f)
		}
	}
	for _, h := range r.Hits.Hits {
		sn := strings.Join(strings.Fields(tagRe.ReplaceAllString(h.Content.Snippet, " ")), " ")
		hits = append(hits, Hit{Repo: h.Repo.Raw, Path: h.Path.Raw, Snippet: sn})
	}
	if total < len(hits) {
		total = len(hits)
	}
	return
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	email := strings.TrimSpace(t.Email)
	var r response
	st, ok := util.GetJSONStatus(ctx, d.HTTP, endpoint+"?q="+url.QueryEscape(email), &r, util.Timeout(30*time.Second))
	if !ok {
		if ctx.Err() == nil {
			msg := "search endpoint unavailable"
			if st != 0 {
				msg = "HTTP " + strconv.Itoa(st)
			}
			emit(models.Finding{Source: "Public code search", Category: "code", Status: models.StatusError, Details: map[string]string{"error": msg}})
		}
		return nil
	}
	hits, total := parse(r)
	// Only files that really contain the address count (the engine is fuzzy on punctuation).
	var real []Hit
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.Snippet), strings.ToLower(email)) {
			real = append(real, h)
		}
	}
	link := "https://grep.app/search?q=" + url.QueryEscape(email)
	if len(real) == 0 {
		emit(models.Finding{Source: "Public code search", Category: "code", URL: link, Status: models.StatusNotFound,
			Details: map[string]string{"query": email, "result": "no public file contains this exact address"}})
		return nil
	}
	repos := map[string]int{}
	var lines []string
	for _, h := range real {
		repos[h.Repo]++
		if len(lines) < 15 {
			lines = append(lines, h.Repo+"/"+h.Path+" — "+util.Clip(h.Snippet, 140))
		}
	}
	names := util.SortedKeys(repos)
	sort.Slice(names, func(i, j int) bool { return repos[names[i]] > repos[names[j]] })
	var ents []models.Entity
	seen := map[string]bool{}
	for _, n := range names {
		owner, _, _ := strings.Cut(n, "/")
		if owner != "" && !seen[owner] && len(ents) < 8 {
			seen[owner] = true
			ents = append(ents, models.Entity{Type: models.EntUsername, Value: owner}) // weak: the repo owner may be someone who merely lists the address
		}
	}
	det := map[string]string{
		"query": email, "files with the address (sample)": strconv.Itoa(len(real)), "total reported": strconv.Itoa(total),
		"repositories": strings.Join(util.ClipList(names, 20), ", "), "matches": strings.Join(lines, "\n"),
		"note": "the address appears in public code; the repository owner may be the subject, a colleague or a project that merely lists contributors",
	}
	emit(models.Finding{Source: "Public code search", Category: "code", URL: link, Status: models.StatusFound, Confidence: 0.6, Details: det, Entities: ents})
	return nil
}
