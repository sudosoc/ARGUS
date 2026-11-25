// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package mentions

import (
	"sort"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/util"
)

func TestIsoDate(t *testing.T) {
	if got := isoDate("20260115T093000Z"); got != "2026-01-15" {
		t.Errorf("got %q", got)
	}
	if got := isoDate("2026"); got != "2026" {
		t.Errorf("short input must pass through, got %q", got)
	}
}

func TestClip(t *testing.T) {
	if got := util.Clip("abc", 5); got != "abc" {
		t.Errorf("got %q", got)
	}
	if got := util.Clip("héllo wörld", 5); got != "héllo wörld" {
		t.Errorf("short values must not be cut: %q", got)
	}
	if got := util.Clip(strings.Repeat("ö", util.MinClip+5), 5); len([]rune(got)) != util.MinClip+1 {
		t.Errorf("rune-safe clip failed: %d", len([]rune(got)))
	}
}

func TestContextWords(t *testing.T) {
	got := contextWords(models.Target{Company: "Acme Robotics Ltd", Location: "Cairo, Egypt", Email: "jane@acmecorp.com"})
	sort.Strings(got)
	has := func(w string) bool {
		for _, g := range got {
			if g == w {
				return true
			}
		}
		return false
	}
	for _, w := range []string{"acme", "robotics", "cairo", "egypt", "acmecorp"} {
		if !has(w) {
			t.Errorf("missing %q in %v", w, got)
		}
	}
	if g := contextWords(models.Target{Email: "jane@gmail.com"}); len(g) != 0 {
		t.Errorf("public mail provider must not become context: %v", g)
	}
	if g := contextWords(models.Target{}); len(g) != 0 {
		t.Errorf("empty target: %v", g)
	}
}
