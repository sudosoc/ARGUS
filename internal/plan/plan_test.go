// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package plan_test

import (
	"testing"

	"github.com/sudosoc/argus/internal/app"
	"github.com/sudosoc/argus/internal/plan"
)

// Every collector must be described (so the plan can say what it reads and where it sends it), and
// every goal may name only collectors that exist.
func TestEveryCollectorIsDescribedAndGoalsAreValid(t *testing.T) {
	o := app.Defaults()
	o.Tools = nil
	b, err := app.Build(o)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, s := range b.Engine.Scanners() {
		known[s.Name()] = true
		m := plan.Of(s.Name())
		if m.Question == "Runs on the details you entered" || len(m.Uses) == 0 {
			t.Errorf("collector %q has no description in internal/plan", s.Name())
		}
	}
	for _, g := range plan.Goals {
		for _, name := range g.Modules {
			if !known[name] {
				t.Errorf("goal %q names unknown collector %q", g.ID, name)
			}
		}
	}
	if plan.GoalByID("").ID != "identify" || plan.GoalByID("nonsense").ID != "identify" {
		t.Error("the default goal should be identify")
	}
	if !plan.GoalByID("everything").Includes("anything") {
		t.Error("everything includes every collector")
	}
}
