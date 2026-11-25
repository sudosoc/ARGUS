// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package leads

import (
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/models"
)

func TestBuildCoversEveryIdentifierAndEscapes(t *testing.T) {
	ls := Build(models.Target{
		FullName: "Jane O'Neil", Username: "jane doe", Email: "a+b@x.org", Phone: "+201001234567",
		Domain: "example.com", IP: "203.0.113.9", ASN: "AS13335", Company: "Acme & Sons", Wallet: "0x00000000219ab540356cBB839Cbe05303d7705Fa",
	})
	groups := map[string]int{}
	for _, l := range ls {
		groups[l.Group]++
		if !strings.HasPrefix(l.URL, "https://") || l.Label == "" || l.Why == "" {
			t.Errorf("malformed lead %+v", l)
		}
		if strings.ContainsAny(l.URL, " \"<>") {
			t.Errorf("unescaped characters in %q", l.URL)
		}
	}
	for _, g := range []string{"username", "name", "image", "email", "phone", "domain", "ip", "asn", "company", "wallet"} {
		if groups[g] == 0 {
			t.Errorf("no manual leads for %s", g)
		}
	}
	if len(Build(models.Target{})) != 0 {
		t.Error("an empty target has no leads")
	}
}

func TestLeadsAreUniquePerGroup(t *testing.T) {
	ls := Build(models.Target{FullName: "Jane Roe", Username: "janeroe", Email: "j@x.org", Phone: "+15551234567", Domain: "x.org", Company: "Acme"})
	seen := map[string]bool{}
	for _, l := range ls {
		k := l.Group + "|" + l.Label
		if seen[k] {
			t.Errorf("duplicate lead %s", k)
		}
		seen[k] = true
	}
	if len(ls) < 60 {
		t.Errorf("expected a rich lead list, got %d", len(ls))
	}
}
