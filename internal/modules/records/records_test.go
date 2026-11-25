// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package records

import "testing"

func TestCleanDisplay(t *testing.T) {
	cases := map[string]string{
		"Doe John  (CIK 0001234567)":  "Doe John",
		"APPLE INC  (CIK 0000320193)": "APPLE INC",
		"Plain Name":                  "Plain Name",
		" , Trim, ":                   "Trim",
	}
	for in, want := range cases {
		if got := cleanDisplay(in); got != want {
			t.Errorf("cleanDisplay(%q) = %q want %q", in, got, want)
		}
	}
}

func TestSortedByCount(t *testing.T) {
	got := sortedByCount(map[string]int{"a": 1, "b": 3, "c": 3, "d": 2}, 3)
	want := []string{"b", "c", "d"} // ties broken alphabetically
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("got %v want %v", got, want)
	}
	if got := sortedByCount(nil, 5); len(got) != 0 {
		t.Errorf("nil: %v", got)
	}
}

func TestTopForms(t *testing.T) {
	if got := topForms(map[string]int{"4": 5, "8-K": 2}); got != "4 ×5, 8-K ×2" {
		t.Errorf("got %q", got)
	}
}
