// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package output

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

func sample() *models.Report {
	return &models.Report{
		Tool: "argus", Version: "t", StartedAt: time.Now(), FinishedAt: time.Now(),
		Target: models.Target{FullName: "Jane Roe", Email: "jane@example.com"},
		Entities: []models.Entity{{Type: models.EntEmail, Value: "jane@example.com", Strong: true, Source: "x"},
			{Type: models.EntDomain, Value: "example.com"}, {Type: models.EntUsername, Value: "jroe"}},
		Findings: []models.Finding{{Module: "people", Source: "ORCID", Category: "person", URL: "javascript:alert(1)", Status: models.StatusFound,
			Confidence: 0.8, Details: map[string]string{"name": "<script>alert(1)</script>"}}},
		Geo: &models.GeoSummary{Consensus: "Cairo, Egypt (90%)", Points: []models.GeoPoint{{Label: "<b>Cairo</b>", Kind: "self-declared", Precision: "city", Weight: 0.7}}},
		Identities: []models.Identity{{ID: 1, Label: "Jane Roe", Score: 0.9, Primary: true,
			Emails: []models.Fact{{Value: "jane@example.com", Sources: []string{"Gravatar"}, Confirmed: true}},
			Docs:   []string{"javascript:alert(2)"}}},
	}
}

func TestAllFormats(t *testing.T) {
	r := sample()
	for _, f := range []string{"json", "csv", "txt", "html", "graphml", "stix", "entities", "misp", "spiderfoot", "maltego", "md", "sql"} {
		var b bytes.Buffer
		if err := Render(&b, r, f); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if b.Len() == 0 {
			t.Fatalf("%s empty", f)
		}
		switch f {
		case "graphml":
			if err := xml.Unmarshal(b.Bytes(), new(struct{ XMLName xml.Name })); err != nil {
				t.Fatalf("graphml not well-formed: %v", err)
			}
		case "stix":
			var v map[string]any
			if err := json.Unmarshal(b.Bytes(), &v); err != nil || v["type"] != "bundle" {
				t.Fatalf("stix invalid: %v", err)
			}
		case "html":
			h := b.String()
			if strings.Contains(h, "<script>alert") || strings.Contains(h, `href="javascript:`) {
				t.Fatal("html report is not escaped")
			}
			if !strings.Contains(h, "Jane Roe") {
				t.Fatal("identity missing")
			}
		}
	}
	if err := Render(&bytes.Buffer{}, r, "bogus"); err == nil {
		t.Fatal("unknown format accepted")
	}
}

func TestLivePDF(t *testing.T) {
	if os.Getenv("ARGUS_LIVE") == "" {
		t.Skip("live")
	}
	var b bytes.Buffer
	if err := Render(&b, sample(), "pdf"); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b.Bytes(), []byte("%PDF")) {
		t.Fatalf("not a pdf: %d bytes", b.Len())
	}
	t.Logf("pdf %d bytes", b.Len())
}

func TestMarkdownIsSafeAndLinked(t *testing.T) {
	r := sample()
	r.Findings = append(r.Findings, models.Finding{Module: "domain", Source: "Site", Category: "web", URL: "https://example.com/x", Status: models.StatusFound, Confidence: 0.9,
		Details: map[string]string{"list": "a\nb"}})
	var b bytes.Buffer
	if err := Render(&b, r, "md"); err != nil {
		t.Fatal(err)
	}
	md := b.String()
	for _, want := range []string{"---\ntool: argus", "# Jane Roe", "[[jane@example.com]]", "[[example.com]]", "[Site](https://example.com/x)", "### web (1)"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
	if strings.Contains(md, "javascript:") {
		t.Error("javascript: links must not reach the note")
	}
	if !strings.Contains(md, "a ¦ b") {
		t.Error("multi-line details must stay on one list line")
	}
}

func TestSQLEscapesAndHasSchema(t *testing.T) {
	r := sample()
	r.Findings[0].Details["name"] = "O'Brien; DROP TABLE findings;--"
	var b bytes.Buffer
	if err := Render(&b, r, "sql"); err != nil {
		t.Fatal(err)
	}
	sql := b.String()
	if !strings.Contains(sql, "'O''Brien; DROP TABLE findings;--'") {
		t.Error("single quotes must be doubled")
	}
	for _, want := range []string{"CREATE TABLE findings", "CREATE TABLE finding_details", "INSERT INTO identities", "BEGIN;", "COMMIT;"} {
		if !strings.Contains(sql, want) {
			t.Errorf("sql lacks %q", want)
		}
	}
}

func TestHTMLReportCarriesTheEmblem(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, sample(), "html"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `src="data:image/webp;base64,`) {
		t.Error("the report must embed the logo as a data URL (html/template would otherwise replace it with #ZgotmplZ)")
	}
	if strings.Contains(b.String(), "ZgotmplZ") {
		t.Error("template sanitiser replaced a URL")
	}
}

func TestInsightsAppearInMarkdownAndHTML(t *testing.T) {
	r := &models.Report{Tool: "argus", Version: "x", Target: models.Target{Username: "lhassan"}, Insights: &models.Insights{
		Brief:   models.Brief{Headline: "Layla Hassan · Cairo", Level: "strong", Paragraphs: []string{"The best match is Layla Hassan."}, Concerns: []string{"Sources disagree about: location"}},
		Actions: []models.Action{{Priority: 90, Title: "Add a location", Why: "two people match", How: "enter a city"}},
	}}
	var md, html strings.Builder
	if err := writeMarkdown(&md, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md.String(), "## Intelligence brief") || !strings.Contains(md.String(), "Add a location") {
		t.Fatalf("markdown: %s", md.String())
	}
	if err := writeHTML(&html, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html.String(), "Intelligence brief") || !strings.Contains(html.String(), "Layla Hassan · Cairo") {
		t.Fatal("html lacks the brief")
	}
}

func TestPostsAppearInMarkdownAndNotPastTheLimit(t *testing.T) {
	r := &models.Report{Target: models.Target{Username: "ann"}}
	r.PostSources = []models.PostSource{{Platform: "Mastodon", Handle: "ann", Count: 2, Oldest: "2026-01-01T00:00:00Z", Newest: "2026-02-01T00:00:00Z"}, {Platform: "Reddit", Handle: "ann", Note: "Reddit did not show this account's history"}}
	for i := 0; i < 45; i++ {
		r.Posts = append(r.Posts, models.Post{Platform: "Mastodon", Kind: "post", Handle: "ann", Time: "2026-02-01T00:00:00Z", Text: "hello world", URL: "https://m.example/1"})
	}
	var b strings.Builder
	writePostsMarkdown(&b, r)
	out := b.String()
	for _, want := range []string{"## Public posts", "without signing in", "Reddit did not show", "and 5 more in the JSON report"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown lacks %q:\n%s", want, out)
		}
	}
}
