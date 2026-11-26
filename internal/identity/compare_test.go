// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package identity

import (
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/models"
)

func fact(v string, src ...string) models.Fact { return models.Fact{Value: v, Sources: src} }

func photo(url, hash string) models.Photo { return models.Photo{URL: url, DHash: hash} }

// Two accounts of one person: different names, different GitHub handles, a similar (not identical) picture.
func TestCompareSecondAccountWithSimilarPicture(t *testing.T) {
	a := models.Identity{ID: 1, Label: "Seif Alsayed", Names: []models.Fact{fact("Seif Alsayed", "GitHub")}, Usernames: []models.Fact{fact("github:programmingman1", "GitHub")},
		Photos: []string{"pa"}, Records: []models.RecordRef{{Platform: "GitHub"}}}
	b := models.Identity{ID: 2, Label: "sudosoc, Seif", Names: []models.Fact{fact("sudosoc, Seif", "GitHub")}, Usernames: []models.Fact{fact("github:sudosoc", "GitHub")},
		Emails: []models.Fact{fact("sudosoc.seif@gmail.com", "GitHub")}, Photos: []string{"pb"}, Records: []models.RecordRef{{Platform: "GitHub"}}}
	// 9 of 64 bits differ
	c := Compare(a, b, []models.Photo{photo("pa", "ffff0000ffff0000"), photo("pb", "ffff0000ffff01ff")})
	if c.Verdict != "likely" && c.Verdict != "possible" {
		t.Fatalf("a similar picture plus a shared first name should not be dismissed: %+v", c)
	}
	var sawPhoto, sawName, sawNote bool
	for _, s := range c.Signals {
		sawPhoto = sawPhoto || (s.Name == "Very similar picture" && s.Effect == "for")
		sawName = sawName || s.Name == "Shared name part"
		sawNote = sawNote || (s.Effect == "note" && strings.Contains(s.Name, "GitHub"))
	}
	if !sawPhoto || !sawName || !sawNote || len(c.Photos) != 1 || c.Photos[0].Distance != 9 || len(c.Advice) == 0 {
		t.Errorf("signals: %+v photos: %+v advice: %v", c.Signals, c.Photos, c.Advice)
	}
}

func TestCompareDifferentPeople(t *testing.T) {
	a := models.Identity{ID: 1, Names: []models.Fact{fact("Seif Alsayed")}, Usernames: []models.Fact{fact("github:programmingman1")}, Photos: []string{"pa"}}
	b := models.Identity{ID: 2, Names: []models.Fact{fact("Laura Fischer")}, Usernames: []models.Fact{fact("github:lfischer")}, Photos: []string{"pb"}}
	c := Compare(a, b, []models.Photo{photo("pa", "0000000000000000"), photo("pb", "ffffffffffffffff")})
	if c.Verdict != "unlikely" || c.Score >= 0.3 {
		t.Fatalf("unrelated people: %+v", c)
	}
	var against bool
	for _, s := range c.Signals {
		against = against || s.Effect == "against"
	}
	if !against {
		t.Error("two full names that share nothing should count against")
	}
}

func TestCompareStrongSignals(t *testing.T) {
	a := models.Identity{ID: 1, Names: []models.Fact{fact("Layla Hassan")}, Emails: []models.Fact{fact("l@example.org")}, Photos: []string{"x"}, Websites: []models.Fact{fact("https://layla.example")}}
	b := models.Identity{ID: 2, Names: []models.Fact{fact("Hassan, Layla")}, Emails: []models.Fact{fact("L@example.org")}, Photos: []string{"x"}, Websites: []models.Fact{fact("layla.example/about")}}
	c := Compare(a, b, nil)
	if c.Verdict != "same" || c.Score < 0.9 {
		t.Fatalf("same name, e-mail, picture and site: %+v", c)
	}
	d := Compare(a, models.Identity{ID: 3, IDs: map[string]string{"orcid": "0000-0001"}}, nil)
	a.IDs = map[string]string{"orcid": "0000-0002"}
	d = Compare(a, models.Identity{ID: 3, IDs: map[string]string{"orcid": "0000-0001"}, Names: []models.Fact{fact("Layla Hassan")}}, nil)
	if d.Verdict != "unlikely" {
		t.Errorf("different ORCID iDs are decisive against: %+v", d)
	}
}

func TestLinkMarksRelatedWithoutMerging(t *testing.T) {
	ids := []models.Identity{
		{ID: 1, Label: "A", Names: []models.Fact{fact("Seif Alsayed")}, Usernames: []models.Fact{fact("github:sudosoc")}, Photos: []string{"pa"}},
		{ID: 2, Label: "B", Names: []models.Fact{fact("Seif A. Alsayed")}, Usernames: []models.Fact{fact("twitter:sudosoc_seif")}, Photos: []string{"pb"}},
		{ID: 3, Label: "C", Names: []models.Fact{fact("Laura Fischer")}, Photos: []string{"pc"}},
	}
	Link(ids, []models.Photo{photo("pa", "ffff0000ffff0000"), photo("pb", "ffff0000ffff0001"), photo("pc", "0000ffff0000ffff")})
	if len(ids) != 3 || len(ids[0].Related) != 1 || ids[0].Related[0].ID != 2 || len(ids[1].Related) != 1 || ids[1].Related[0].ID != 1 {
		t.Fatalf("related: %+v / %+v", ids[0].Related, ids[1].Related)
	}
	if len(ids[2].Related) != 0 {
		t.Errorf("an unrelated person must not be linked: %+v", ids[2].Related)
	}
}
