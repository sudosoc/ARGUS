// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package lookalike

import (
	"context"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestSplit(t *testing.T) {
	for in, want := range map[string][2]string{"example.com": {"example", "com"}, "www.shop.example.co.uk": {"example", "co.uk"}, "a.b.example.org": {"example", "org"}} {
		s, tl, ok := Split(in)
		if !ok || s != want[0] || tl != want[1] {
			t.Errorf("Split(%s) = %s %s %v", in, s, tl, ok)
		}
	}
	if _, _, ok := Split("localhost"); ok {
		t.Error("single label")
	}
}

func TestGenerateCoversTechniques(t *testing.T) {
	c := Generate("example.com", 0)
	if len(c) < 150 {
		t.Fatalf("only %d candidates", len(c))
	}
	techs := Techniques(c)
	for _, want := range []string{"omission", "transposition", "repetition", "keyboard typo", "hyphenation", "look-alike character", "IDN homograph", "bit-squatting", "other TLD", "added keyword"} {
		if techs[want] == 0 {
			t.Errorf("technique %q produced nothing (%v)", want, SortedTechniques(techs))
		}
	}
	seen := map[string]bool{}
	for _, x := range c {
		if x.Domain == "example.com" {
			t.Error("the original must not be listed")
		}
		if seen[x.Domain] {
			t.Errorf("duplicate %s", x.Domain)
		}
		seen[x.Domain] = true
		if strings.ContainsAny(x.Domain, " _") || len(x.Domain) > 253 {
			t.Errorf("invalid name %q", x.Domain)
		}
	}
	if !seen["exmple.com"] || !seen["examlpe.com"] || !seen["example.net"] || !seen["example-login.com"] {
		t.Error("expected common typo/TLD/keyword candidates missing")
	}
}

func TestPunycodeKnownVectors(t *testing.T) {
	// RFC 3492 / IDNA examples.
	for in, want := range map[string]string{"bücher": "bcher-kva", "münchen": "mnchen-3ya", "пример": "e1afmkfd"} {
		got, _ := punycode(in)
		if got != want {
			t.Errorf("punycode(%s) = %s want %s", in, got, want)
		}
	}
	if a, _ := toASCII("exаmple.com"); !strings.HasPrefix(a, "xn--") { // Cyrillic а
		t.Errorf("IDN must convert: %s", a)
	}
}

func TestMaxIsHonoured(t *testing.T) {
	if got := Generate("example.com", 25); len(got) != 25 {
		t.Errorf("len %d", len(got))
	}
	if Generate("a.com", 10) != nil {
		t.Error("one-letter names have no useful variants")
	}
}

func TestScanReportsOnlyResolving(t *testing.T) {
	oldR, oldM := resolveHost, hasMX
	defer func() { resolveHost, hasMX = oldR, oldM }()
	resolveHost = func(_ context.Context, h string) ([]string, error) {
		switch h {
		case "exmple.com", "example-login.com":
			return []string{"203.0.113.9"}, nil
		}
		return nil, context.DeadlineExceeded
	}
	hasMX = func(_ context.Context, h string) bool { return h == "example-login.com" }
	p := pool.New(8, 8)
	defer p.Close()
	var got []models.Finding
	err := New().Scan(context.Background(), models.Target{Domain: "https://www.Example.com/path"}, scanner.Deps{Pool: p}, func(f models.Finding) { got = append(got, f) })
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	f := got[0]
	if f.Status != models.StatusFound || f.Details["registered look-alikes"] != "2" || f.Details["able to receive mail"] != "1" {
		t.Errorf("%+v", f.Details)
	}
	if !strings.HasPrefix(strings.Split(f.Details["look-alikes"], "\n")[0], "example-login.com") {
		t.Errorf("mail-capable look-alikes must come first: %s", f.Details["look-alikes"])
	}
	if len(f.Entities) != 1 || f.Entities[0].Value != "example-login.com" {
		t.Errorf("entities: %+v", f.Entities)
	}
}

func TestCanRun(t *testing.T) {
	s := New()
	if !s.CanRun(models.Target{Domain: "example.com"}) || s.CanRun(models.Target{Domain: "gmail.com"}) || s.CanRun(models.Target{Username: "x"}) {
		t.Error("CanRun")
	}
}
