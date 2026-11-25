package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// writeMarkdown renders the case as one Markdown note that drops straight into
// Obsidian or Logseq: YAML front matter, [[wikilinks]] for people, domains and
// accounts so the case joins an existing knowledge graph, and the same
// sections as the report.
func writeMarkdown(w io.Writer, r *models.Report) error {
	var b strings.Builder
	title := targetTitle(r.Target)
	fmt.Fprintf(&b, "---\ntool: %s\nversion: %s\ntarget: %q\nstarted: %s\nfinished: %s\n", r.Tool, r.Version, title,
		r.StartedAt.UTC().Format(time.RFC3339), r.FinishedAt.UTC().Format(time.RFC3339))
	if r.Integrity != nil {
		fmt.Fprintf(&b, "sha256: %s\nfindings: %d\n", r.Integrity.Digest, r.Integrity.Findings)
	}
	if r.Interrupted {
		b.WriteString("partial: true\n")
	}
	b.WriteString("tags: [osint, argus]\n---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	if r.Interrupted {
		b.WriteString("> The run was interrupted: results are partial.\n\n")
	}
	if r.Ambiguity != "" {
		fmt.Fprintf(&b, "> **Ambiguity:** %s\n\n", r.Ambiguity)
	}

	if len(r.Identities) > 0 {
		b.WriteString("## Identity\n\n")
		for _, id := range r.Identities {
			mark := ""
			if id.Primary {
				mark = " ★ best match"
			}
			fmt.Fprintf(&b, "### %s — %.0f%%%s\n\n", id.Label, id.Score*100, mark)
			if len(id.Matched) > 0 {
				fmt.Fprintf(&b, "- matches your input: %s\n", strings.Join(id.Matched, ", "))
			}
			facts := func(label string, fs []models.Fact, link bool) {
				if len(fs) == 0 {
					return
				}
				var parts []string
				for _, f := range fs {
					v := f.Value
					if link {
						v = "[[" + v + "]]"
					}
					if f.Confirmed {
						v += " ✔"
					}
					parts = append(parts, v+" ("+strings.Join(f.Sources, ", ")+")")
				}
				fmt.Fprintf(&b, "- %s: %s\n", label, strings.Join(parts, "; "))
			}
			facts("names", id.Names, true)
			facts("headlines", id.Headlines, false)
			facts("employers", id.Employers, true)
			facts("locations", id.Locations, false)
			facts("education", id.Education, true)
			facts("e-mails", id.Emails, true)
			facts("phones", id.Phones, false)
			facts("accounts", id.Usernames, false)
			facts("websites", id.Websites, false)
			if id.Bio != "" {
				fmt.Fprintf(&b, "- bio: %s\n", oneLine(id.Bio, 400))
			}
			for _, c := range id.Conflicts {
				fmt.Fprintf(&b, "- ⚠ conflict: %s\n", c)
			}
			b.WriteString("\n")
		}
	}

	if ents := flatEntities(r); len(ents) > 0 {
		b.WriteString("## Entities\n\n")
		by := map[string][]string{}
		var kinds []string
		for _, e := range ents {
			if _, ok := by[e.typ]; !ok {
				kinds = append(kinds, e.typ)
			}
			v := e.val
			if e.typ == models.EntDomain || e.typ == models.EntName || e.typ == models.EntOrg || e.typ == models.EntEmail || e.typ == models.EntUsername {
				v = "[[" + v + "]]"
			}
			by[e.typ] = append(by[e.typ], v)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			fmt.Fprintf(&b, "- **%s**: %s\n", k, strings.Join(by[k], ", "))
		}
		b.WriteString("\n")
	}

	if r.Geo != nil && (r.Geo.Consensus != "" || len(r.Geo.Clusters) > 0) {
		b.WriteString("## Location\n\n")
		if r.Geo.Consensus != "" {
			fmt.Fprintf(&b, "- most likely: **%s**\n", r.Geo.Consensus)
		}
		if r.Geo.Conflict != "" {
			fmt.Fprintf(&b, "- ⚠ %s\n", r.Geo.Conflict)
		}
		if tz := r.Geo.TZ; tz != nil {
			fmt.Fprintf(&b, "- time zone from activity: %s (%.0f%% confidence, %d events)\n", tz.Label, tz.Confidence*100, tz.Events)
		}
		b.WriteString("\n")
	}

	hits := r.Hits()
	byCat := map[string][]models.Finding{}
	var cats []string
	for _, f := range hits {
		c := f.Category
		if c == "" {
			c = "other"
		}
		if _, ok := byCat[c]; !ok {
			cats = append(cats, c)
		}
		byCat[c] = append(byCat[c], f)
	}
	sort.Strings(cats)
	fmt.Fprintf(&b, "## Findings (%d)\n\n", len(hits))
	for _, c := range cats {
		fmt.Fprintf(&b, "### %s (%d)\n\n", c, len(byCat[c]))
		for _, f := range byCat[c] {
			link := f.Source
			if isWebURL(f.URL) {
				link = "[" + f.Source + "](" + f.URL + ")"
			}
			fmt.Fprintf(&b, "- %s — %s, %.0f%%\n", link, f.Status, f.Confidence*100)
			for _, l := range detailLines(f.Details, 8) {
				fmt.Fprintf(&b, "  - %s\n", l)
			}
		}
		b.WriteString("\n")
	}

	if len(r.Timeline) > 0 {
		b.WriteString("## Timeline\n\n")
		for _, e := range r.Timeline {
			fmt.Fprintf(&b, "- %s — %s (%s)\n", e.Date, e.Event, e.Source)
		}
		b.WriteString("\n")
	}
	if len(r.Notes) > 0 {
		b.WriteString("## Analyst notes\n\n")
		keys := make([]string, 0, len(r.Notes))
		for k := range r.Notes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- **%s:** %s\n", noteTitle(k), oneLine(r.Notes[k], 2000))
		}
		b.WriteString("\n")
	}
	if len(r.ManualLeads) > 0 {
		b.WriteString("## Manual leads (need a login, CAPTCHA or payment)\n\n")
		for _, l := range r.ManualLeads {
			if !isWebURL(l.URL) {
				continue
			}
			fmt.Fprintf(&b, "- [ ] %s: [%s](%s) — %s\n", l.Group, l.Label, l.URL, l.Why)
		}
		b.WriteString("\n")
	}
	if len(r.ToolRuns) > 0 {
		b.WriteString("## Raw tool output\n\nThe complete output of every tool is stored in the JSON report (`tool_runs`) and in the `.tools` folder next to the exported files:\n\n")
		for _, tr := range r.ToolRuns {
			fmt.Fprintf(&b, "- %s: %d bytes\n", tr.Tool, len(tr.Output))
		}
		b.WriteString("\n")
	}
	if len(r.Failures) > 0 {
		b.WriteString("## Coverage gaps\n\nA check that could not complete is not proof that nothing exists.\n\n")
		for _, f := range r.Failures {
			fmt.Fprintf(&b, "- %s / %s: %s\n", f.Module, f.Source, oneLine(f.Error, 200))
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func targetTitle(t models.Target) string {
	for _, v := range []string{t.FullName, t.Username, t.Email, t.Domain, t.Company, t.Phone, t.IP, t.Alias, t.ASN, t.Wallet} {
		if v != "" {
			return v
		}
	}
	return "case"
}

// isWebURL keeps javascript:, data: and file: links out of the note.
func isWebURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

// writeSQL renders the case as a SQL script (SQLite dialect) that creates and
// fills tables for findings, their details, entities, identities, photos,
// leads and gaps. `sqlite3 case.db < case.sql` then `datasette case.db` turns
// it into a browsable, queryable database.
func writeSQL(w io.Writer, r *models.Report) error {
	var b strings.Builder
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	b.WriteString(`-- SUDOSOC ARGUS case export (SQLite). Load with: sqlite3 case.db < this-file.sql
PRAGMA foreign_keys = ON;
BEGIN;
CREATE TABLE report (tool TEXT, version TEXT, target_json TEXT, started TEXT, finished TEXT, interrupted INTEGER, sha256 TEXT);
CREATE TABLE findings (id INTEGER PRIMARY KEY, module TEXT, source TEXT, category TEXT, url TEXT, status TEXT, confidence REAL, via TEXT, found_at TEXT);
CREATE TABLE finding_details (finding_id INTEGER REFERENCES findings(id), key TEXT, value TEXT);
CREATE TABLE entities (type TEXT, value TEXT, strong INTEGER, source TEXT);
CREATE TABLE identities (id INTEGER PRIMARY KEY, label TEXT, score REAL, is_primary INTEGER, weak INTEGER, bio TEXT);
CREATE TABLE identity_facts (identity_id INTEGER REFERENCES identities(id), kind TEXT, value TEXT, confirmed INTEGER, sources TEXT);
CREATE TABLE photos (url TEXT, sha256 TEXT, dhash TEXT, width INTEGER, height INTEGER, grp INTEGER, sources TEXT, gps TEXT, taken TEXT, camera TEXT);
CREATE TABLE timeline (date TEXT, event TEXT, source TEXT, url TEXT);
CREATE TABLE leads (grp TEXT, label TEXT, url TEXT, why TEXT);
CREATE TABLE gaps (module TEXT, source TEXT, error TEXT);
CREATE TABLE notes (key TEXT, text TEXT);
CREATE TABLE tool_runs (tool TEXT, command TEXT, output TEXT, stderr TEXT, error TEXT, bytes INTEGER, truncated INTEGER, duration_ms INTEGER);
CREATE INDEX findings_category ON findings(category);
CREATE INDEX findings_status ON findings(status);
CREATE INDEX details_finding ON finding_details(finding_id);
`)
	tj, _ := json.Marshal(r.Target)
	sum := ""
	if r.Integrity != nil {
		sum = r.Integrity.Digest
	}
	fmt.Fprintf(&b, "INSERT INTO report VALUES (%s,%s,%s,%s,%s,%d,%s);\n", q(r.Tool), q(r.Version), q(string(tj)),
		q(r.StartedAt.UTC().Format(time.RFC3339)), q(r.FinishedAt.UTC().Format(time.RFC3339)), b2i(r.Interrupted), q(sum))
	for i, f := range r.Findings {
		id := i + 1
		fmt.Fprintf(&b, "INSERT INTO findings VALUES (%d,%s,%s,%s,%s,%s,%g,%s,%s);\n", id, q(f.Module), q(f.Source), q(f.Category), q(f.URL),
			q(string(f.Status)), f.Confidence, q(f.Via), q(f.Timestamp.UTC().Format(time.RFC3339)))
		keys := make([]string, 0, len(f.Details))
		for k := range f.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "INSERT INTO finding_details VALUES (%d,%s,%s);\n", id, q(k), q(f.Details[k]))
		}
	}
	for _, e := range r.Entities {
		fmt.Fprintf(&b, "INSERT INTO entities VALUES (%s,%s,%d,%s);\n", q(e.Type), q(e.Value), b2i(e.Strong), q(e.Source))
	}
	for _, id := range r.Identities {
		fmt.Fprintf(&b, "INSERT INTO identities VALUES (%d,%s,%g,%d,%d,%s);\n", id.ID, q(id.Label), id.Score, b2i(id.Primary), b2i(id.Weak), q(id.Bio))
		add := func(kind string, fs []models.Fact) {
			for _, f := range fs {
				fmt.Fprintf(&b, "INSERT INTO identity_facts VALUES (%d,%s,%s,%d,%s);\n", id.ID, q(kind), q(f.Value), b2i(f.Confirmed), q(strings.Join(f.Sources, ", ")))
			}
		}
		add("name", id.Names)
		add("headline", id.Headlines)
		add("employer", id.Employers)
		add("location", id.Locations)
		add("education", id.Education)
		add("interest", id.Interests)
		add("email", id.Emails)
		add("phone", id.Phones)
		add("username", id.Usernames)
		add("website", id.Websites)
	}
	for _, p := range r.Photos {
		fmt.Fprintf(&b, "INSERT INTO photos VALUES (%s,%s,%s,%d,%d,%d,%s,%s,%s,%s);\n", q(p.URL), q(p.SHA256), q(p.DHash), p.Width, p.Height, p.Group,
			q(strings.Join(p.Sources, ", ")), q(p.GPS), q(p.Taken), q(p.Camera))
	}
	for _, e := range r.Timeline {
		fmt.Fprintf(&b, "INSERT INTO timeline VALUES (%s,%s,%s,%s);\n", q(e.Date), q(e.Event), q(e.Source), q(e.URL))
	}
	for _, l := range r.ManualLeads {
		fmt.Fprintf(&b, "INSERT INTO leads VALUES (%s,%s,%s,%s);\n", q(l.Group), q(l.Label), q(l.URL), q(l.Why))
	}
	for _, f := range r.Failures {
		fmt.Fprintf(&b, "INSERT INTO gaps VALUES (%s,%s,%s);\n", q(f.Module), q(f.Source), q(f.Error))
	}
	for _, tr := range r.ToolRuns {
		fmt.Fprintf(&b, "INSERT INTO tool_runs VALUES (%s,%s,%s,%s,%s,%d,%d,%d);\n", q(tr.Tool), q(tr.Command), q(tr.Output), q(tr.Stderr), q(tr.Error), tr.Bytes, b2i(tr.Truncated), tr.Millis)
	}
	for k, v := range r.Notes {
		fmt.Fprintf(&b, "INSERT INTO notes VALUES (%s,%s);\n", q(k), q(v))
	}
	b.WriteString("COMMIT;\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
