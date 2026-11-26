// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package sensitive

import "testing"

func TestMatchProtected(t *testing.T) {
	for _, s := range []string{
		"Proud Muslim and father of two", "Devout Christian, pastor at heart", "Jewish heritage", "praise the Lord",
		"Living with cancer, sharing my journey", "mental health advocate", "type 1 diabetic", "diagnosed with ADHD",
		"r/atheism", "r/lgbt", "Gay rights", "proud lesbian", "LGBTQ+ ally", "non-binary writer", "she/her",
		"Democrat, vote for change", "Conservative party member", "left-wing activist", "trade union rep", "MAGA",
		"مسلم وأفتخر", "في المسجد كل جمعة", "حزب الوفد", "ناشط سياسي", "مصاب بالسرطان", "الكنيسة", "مثلي",
	} {
		if !Is(s) {
			t.Errorf("should be withheld: %q", s)
		}
	}
}

func TestMatchHarmless(t *testing.T) {
	for _, s := range []string{
		"Go, Python, security", "Security engineer at Hassan Labs", "Cairo, Egypt", "Liberal arts college",
		"keystroke dynamics", "Chronicle of events", "Digital forensics and incident response", "open source maintainer",
		"Godot game engine", "Christchurch", "Labor market economics", "PhD candidate", "progressive web apps", "نائب مدير الشركة",
		"عمل في البنك القومي", "مهندس أمن معلومات", "Trumpet player",
	} {
		if Is(s) {
			t.Errorf("should be kept: %q (%v)", s, Match(s))
		}
	}
}

func TestKeepAndScrub(t *testing.T) {
	kept, n := Keep([]string{"Go", "Islamic studies", "Python", "politics"})
	if len(kept) != 2 || n != 2 {
		t.Fatalf("kept=%v n=%d", kept, n)
	}
	type inner struct{ Note string }
	v := struct {
		A string
		B []string
		C *inner
		D map[string]string
		e string
	}{A: "fine", B: []string{"Go", "queer rights"}, C: &inner{Note: "prays daily"}, D: map[string]string{"k": "voted for the party", "j": "ok"}, e: "private"}
	if got := Scrub(&v); got != 3 {
		t.Fatalf("scrubbed %d, want 3: %+v", got, v)
	}
	if v.B[1] != Withheld || v.C.Note != Withheld || v.D["k"] != Withheld || v.A != "fine" || v.D["j"] != "ok" || v.B[0] != "Go" {
		t.Fatalf("unexpected %+v", v)
	}
}
