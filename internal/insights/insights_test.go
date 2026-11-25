// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

func fixture() *models.Report {
	now := time.Now().UTC()
	recent := now.AddDate(0, 0, -10).Format("2006-01-02")
	rec := func(platform, name, bio, photo string, handles ...string) *models.Record {
		return &models.Record{Platform: platform, URL: "https://" + strings.ToLower(platform) + ".example/" + name, Name: name, Bio: bio, Photo: photo, Usernames: handles, Self: platform == "GitHub"}
	}
	r1 := rec("GitHub", "Layla Hassan", "Security engineer in Cairo. I write about incident response and detection engineering for small teams.", "p1", "GitHub:lhassan")
	r2 := rec("Keybase", "Layla Hassan", "", "p1", "Keybase:l.hassan")
	r3 := rec("ORCID", "Layla Hassan", "", "")
	r4 := rec("Mastodon", "Dark Knight", "Security engineer in Cairo. I write about incident response and detection engineering for small teams.", "p1", "Mastodon:lhassan89")
	var tz models.TZEstimate
	tz.Offset, tz.Label, tz.Events = 2, "UTC+02:00", 400
	for h := 0; h < 24; h++ {
		tz.Hours[h] = 10
	}
	for _, h := range []int{22, 23, 0, 1, 2, 3} { // UTC 22..03 = local 00..05: quiet
		tz.Hours[h] = 0
	}
	tz.Hours[14] = 80 // local 16:00 peak
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			tz.Grid[d][h] = tz.Hours[h] / 7
		}
	}
	rep := &models.Report{
		Target: models.Target{FullName: "Layla Hassan", Username: "lhassan"},
		Identities: []models.Identity{{
			ID: 1, Label: "Layla Hassan", Score: 0.87, Primary: true,
			Names:     []models.Fact{{Value: "Layla Hassan", Sources: []string{"GitHub", "Keybase", "ORCID"}, Confirmed: true}},
			Headlines: []models.Fact{{Value: "Security engineer", Sources: []string{"GitHub"}, Self: true}},
			Employers: []models.Fact{{Value: "Hassan Labs", Sources: []string{"GitHub", "ORCID"}, Confirmed: true}},
			Locations: []models.Fact{{Value: "Cairo, Egypt", Sources: []string{"GitHub"}, Self: true}},
			Emails:    []models.Fact{{Value: "lhassan@hassanlabs.org", Sources: []string{"GitHub"}, Self: true}},
			Usernames: []models.Fact{{Value: "GitHub:lhassan", Sources: []string{"GitHub"}}},
			Education: []models.Fact{{Value: "American University in Cairo", Sources: []string{"ORCID"}}},
			Bio:       r1.Bio,
			Conflicts: []string{"location"},
			Records:   []models.RecordRef{{Platform: "GitHub", URL: r1.URL}, {Platform: "Keybase", URL: r2.URL}, {Platform: "ORCID", URL: r3.URL}, {Platform: "Mastodon", URL: r4.URL}},
		}},
		Photos: []models.Photo{
			{URL: "p1", Sources: []string{"GitHub", "Keybase", "Mastodon"}, Group: 1, DHash: "ffff0000ffff0000", GPS: "30.0444,31.2357", Taken: "2022:05:01 10:00:00"},
			{URL: "p2", Sources: []string{"x"}, DHash: "ffff0000ffff0007", GPS: "51.5074,-0.1278", Taken: "2023:03:01 10:00:00"},
		},
		Timeline: []models.TimelineEvent{
			{Date: "2012-01-05", Event: "account created", Source: "GitHub profile"},
			{Date: "2012-01-20", Event: "account created", Source: "Keybase"},
			{Date: "2012-01-30", Event: "account created", Source: "Mastodon"},
			{Date: "2018-06-01", Event: "account created", Source: "ORCID"},
			{Date: recent, Event: "last public activity seen", Source: "GitHub"},
		},
		Geo: &models.GeoSummary{Consensus: "Cairo, Egypt", TZ: &tz, Points: []models.GeoPoint{{Label: "Cairo, Egypt", Kind: "self-declared", Lat: 30.04, Lon: 31.23, Country: "Egypt"}}},
		Findings: []models.Finding{
			{Module: "profiles", Source: "GitHub", Status: models.StatusFound, URL: r1.URL, Record: r1, Details: map[string]string{"created": "2012-01-05"}},
			{Module: "profiles", Source: "Keybase", Status: models.StatusFound, URL: r2.URL, Record: r2},
			{Module: "people", Source: "ORCID", Status: models.StatusFound, URL: r3.URL, Record: r3},
			{Module: "profiles", Source: "Mastodon", Status: models.StatusFound, URL: r4.URL, Record: r4},
			{Module: "social", Source: "Reddit profile", Status: models.StatusFound, URL: "https://reddit.example/lhassan", Details: map[string]string{"username": "lhassan"}},
			{Module: "social", Source: "Twitter profile", Status: models.StatusFound, URL: "https://twitter.example/lhassan", Details: map[string]string{"username": "lhassan"}},
			{Module: "social", Source: "Steam profile", Status: models.StatusFound, URL: "https://steam.example/lhassan", Details: map[string]string{"username": "lhassan"}},
			{Module: "devtrace", Source: "GitHub activity profile", Status: models.StatusFound, Details: map[string]string{"main languages": "Go (12), Python (5)", "topics": "dfir (3), osint (2)", "public organisations": "hassan-labs", "authors in public commits": "Layla <l@home.example> (12)"}},
			{Module: "bskygraph", Source: "Bluesky social circle", Status: models.StatusFound, Details: map[string]string{"mutual follows": "Sam Ali (@sam.example); Dina Nour (@dina.example)", "likely connections": "Omar K (@omar.example) — followed by 6 of the 10 accounts checked"}},
			{Module: "breach", Category: "breach", Source: "Breaches", Status: models.StatusFound, Details: map[string]string{"breach names": "Adobe"}},
		},
		ManualLeads: []models.Lead{{Group: "name", Label: "LinkedIn people search", URL: "https://linkedin.example/", Why: "people search"}},
		Modules:     []models.ModuleRun{{Name: "social", Checked: 100, Errors: 30}},
	}
	return rep
}

func TestBuildProducesEverySection(t *testing.T) {
	in := Build(fixture())
	if in.Brief.Level != "strong" && in.Brief.Level != "moderate" {
		t.Fatalf("level %q", in.Brief.Level)
	}
	if !strings.Contains(in.Brief.Headline, "Layla Hassan") || !strings.Contains(in.Brief.Headline, "Cairo") {
		t.Fatalf("headline %q", in.Brief.Headline)
	}
	if len(in.Brief.Paragraphs) < 3 {
		t.Fatalf("paragraphs %v", in.Brief.Paragraphs)
	}
	if in.Persona == nil || in.Authenticity == nil || in.Life == nil || in.Movement == nil || len(in.Actions) == 0 || len(in.Explain) == 0 {
		t.Fatalf("missing section: %+v", in)
	}
}

func TestPersona(t *testing.T) {
	p := Build(fixture()).Persona
	if len(p.Languages) == 0 || p.Languages[0].Label != "English" {
		t.Fatalf("languages %+v", p.Languages)
	}
	if p.Schedule == nil || p.Schedule.SleepFrom != 0 || p.Schedule.PeakHour != 16 {
		t.Fatalf("schedule %+v", p.Schedule)
	}
	if p.Expertise[0].Label != "Go" || p.FirstTrace != "2012-01-05" || p.FirstSource != "GitHub profile" {
		t.Fatalf("expertise/first %+v %q", p.Expertise, p.FirstTrace)
	}
	joined := strings.Join(p.Exposure, " | ")
	for _, want := range []string{"handle is reused", "same profile picture", "GPS", "commits", "breaches"} {
		if !strings.Contains(joined, want) {
			t.Errorf("exposure missing %q in %s", want, joined)
		}
	}
}

func TestArabicLanguageDetected(t *testing.T) {
	l := languages([]string{"مهندسة أمن معلومات أكتب عن الاستجابة للحوادث", "Security engineer in Cairo working with the team on detection and response"})
	if len(l) != 2 || l[0].Label != "Arabic" && l[1].Label != "Arabic" {
		t.Fatalf("%+v", l)
	}
}

func TestAuthenticityFactorsAddUp(t *testing.T) {
	a := Build(fixture()).Authenticity
	sum := 50
	for _, f := range a.Factors {
		sum += f.Effect
	}
	if clamp(sum, 0, 100) != a.Score {
		t.Fatalf("score %d does not equal 50 + factors %d", a.Score, sum)
	}
	found := false
	for _, f := range a.Factors {
		if f.Name == "Picture reuse" && f.Effect < 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("picture reuse under another name not penalised: %+v", a.Factors)
	}
}

func TestAssociatesRankedAndCombined(t *testing.T) {
	as := Build(fixture()).Associates
	if len(as) < 4 {
		t.Fatalf("%+v", as)
	}
	names := map[string]bool{}
	for _, a := range as {
		names[a.Name] = true
	}
	if !names["Hassan Labs"] || !names["American University in Cairo"] {
		t.Fatalf("missing org/school: %v", names)
	}
	if as[0].Strength < as[len(as)-1].Strength {
		t.Fatal("not sorted by strength")
	}
}

func TestHiddenLinks(t *testing.T) {
	hl := Build(fixture()).HiddenLinks
	kinds := map[string]bool{}
	for _, l := range hl {
		kinds[l.Kind] = true
	}
	for _, k := range []string{"handle pattern", "similar text", "similar picture"} {
		if !kinds[k] {
			t.Errorf("no %q link in %+v", k, hl)
		}
	}
}

func TestLifeBurstGapRecent(t *testing.T) {
	l := Build(fixture()).Life
	kinds := map[string]bool{}
	for _, a := range l.Anomalies {
		kinds[a.Kind] = true
	}
	if !kinds["burst"] || !kinds["gap"] || !kinds["recent"] {
		t.Fatalf("anomalies %+v", l.Anomalies)
	}
	if l.First != "2012-01-05" {
		t.Fatalf("first %s", l.First)
	}
}

func TestMovementTrip(t *testing.T) {
	m := Build(fixture()).Movement
	if len(m.Trips) != 1 || m.Trips[0].Km < 3000 || m.Trips[0].Date != "2023-03-01" {
		t.Fatalf("trips %+v", m.Trips)
	}
	if m.Home != "Cairo, Egypt" {
		t.Fatalf("home %q", m.Home)
	}
}

func TestExplainLevels(t *testing.T) {
	var name, loc, mail models.Explanation
	for _, e := range Build(fixture()).Explain {
		switch e.Attribute {
		case "name":
			name = e
		case "location":
			loc = e
		case "e-mail":
			mail = e
		}
	}
	if name.Level != "confirmed" || loc.Level != "contested" || mail.Level != "self-reported" {
		t.Fatalf("levels %q %q %q", name.Level, loc.Level, mail.Level)
	}
}

func TestActionsSortedAndUseful(t *testing.T) {
	acts := Build(fixture()).Actions
	if len(acts) == 0 {
		t.Fatal("no actions")
	}
	for i := 1; i < len(acts); i++ {
		if acts[i].Priority > acts[i-1].Priority {
			t.Fatal("not sorted")
		}
	}
	titles := ""
	for _, a := range acts {
		titles += a.Title + "|"
	}
	for _, want := range []string{"impersonator", "Resolve: location", "Re-run later", "LinkedIn"} {
		if !strings.Contains(titles, want) {
			t.Errorf("no action containing %q in %s", want, titles)
		}
	}
}

func TestEmptyReportIsHandled(t *testing.T) {
	in := Build(&models.Report{})
	if in.Brief.Level != "none" || len(in.Actions) == 0 || len(in.Limits) == 0 {
		t.Fatalf("%+v", in)
	}
	if in.Persona != nil || in.Authenticity != nil {
		t.Fatal("sections should be nil for an empty report")
	}
}

func TestDeterministic(t *testing.T) {
	a, b := Build(fixture()), Build(fixture())
	if len(a.Actions) != len(b.Actions) || a.Brief.Headline != b.Brief.Headline || a.Authenticity.Score != b.Authenticity.Score {
		t.Fatal("not deterministic")
	}
}

// Nothing derived may state religion, health, sexual orientation or political affiliation,
// even when the evidence itself does.
func TestProtectedCharacteristicsNeverDerived(t *testing.T) {
	rep := fixture()
	id := &rep.Identities[0]
	id.Interests = []models.Fact{{Value: "Islamic history", Sources: []string{"GitHub"}}, {Value: "chess", Sources: []string{"GitHub"}}}
	id.Headlines = append(id.Headlines, models.Fact{Value: "Labour party activist", Sources: []string{"Mastodon"}})
	id.Employers = append(id.Employers, models.Fact{Value: "St Mark Coptic Church", Sources: []string{"ORCID"}})
	id.Education = append(id.Education, models.Fact{Value: "Cairo Institute of Theology", Sources: []string{"ORCID"}})
	rep.Timeline = append(rep.Timeline, models.TimelineEvent{Date: "2020-02-02", Event: "joined a cancer support group", Source: "Forum"})
	rep.Findings = append(rep.Findings, models.Finding{Module: "devtrace", Source: "GitHub activity profile", Status: models.StatusFound,
		Details: map[string]string{"topics": "lgbtq (4), osint (2)", "main languages": "Go (9)"}})
	in := Build(rep)
	b, _ := json.Marshal(in)
	text := strings.ToLower(string(b))
	for _, w := range []string{"islamic", "labour party", "coptic", "theology", "cancer", "lgbtq"} {
		if strings.Contains(text, w) {
			t.Errorf("derived output mentions %q: %s", w, text)
		}
	}
	for _, w := range []string{"chess", "osint", "go"} {
		if !strings.Contains(text, w) {
			t.Errorf("harmless %q was lost", w)
		}
	}
	if len(in.Limits) == 0 || !strings.Contains(in.Limits[0], "left out") {
		t.Errorf("the withheld items are not accounted for: %v", in.Limits)
	}
}
