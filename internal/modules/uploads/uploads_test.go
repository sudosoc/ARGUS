// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package uploads

import (
	"context"
	"testing"

	"github.com/sudosoc/argus/internal/files"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestScanEmitsFindingWithLeads(t *testing.T) {
	a, err := files.Analyze(context.Background(), "note.txt", []byte("Contact layla@hassanlabs.org +20 100 555 0199 https://hassanlabs.org"))
	if err != nil {
		t.Fatal(err)
	}
	var got []models.Finding
	s := New()
	tg := models.Target{Files: []string{a.ID, "missing"}}
	if !s.CanRun(tg) || s.CanRun(models.Target{}) {
		t.Fatal("CanRun wrong")
	}
	if err := s.Scan(context.Background(), tg, scanner.Deps{}, func(f models.Finding) { got = append(got, f) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Category != "file" || got[1].Status != models.StatusError {
		t.Fatalf("%+v", got)
	}
	kinds := map[string]bool{}
	for _, e := range got[0].Entities {
		kinds[e.Type] = true
	}
	if !kinds[models.EntEmail] || !kinds[models.EntPhone] || !kinds[models.EntDomain] {
		t.Fatalf("entities %+v", got[0].Entities)
	}
}
