// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package activity

import (
	"testing"
	"time"
)

func stamps(n int, hour int) []time.Time {
	base := time.Date(2026, 3, 20, hour, 0, 0, 0, time.UTC)
	out := make([]time.Time, n)
	for i := range out { // newest first, as the collectors return them
		out[i] = base.AddDate(0, 0, -i)
	}
	return out
}

func TestFindingNeedsEnoughEvents(t *testing.T) {
	if _, ok := finding("GitHub", "bob", stamps(9, 10)); ok {
		t.Fatal("9 events are too few")
	}
}

func TestFindingDetails(t *testing.T) {
	f, ok := finding("GitHub", "bob", stamps(30, 10))
	if !ok {
		t.Fatal("expected a finding")
	}
	if f.Source != "GitHub activity hours" || f.Category != "activity" {
		t.Errorf("source/category: %+v", f)
	}
	d := f.Details
	if d["events"] != "30" || d["handle"] != "bob" {
		t.Errorf("details: %v", d)
	}
	if d["last event"] != "2026-03-20" || d["first event"] != "2026-02-19" {
		t.Errorf("event range: first=%s last=%s", d["first event"], d["last event"])
	}
	if len(splitCSV(d["hours_utc"])) != 24 || len(splitCSV(d["week_utc"])) != 168 {
		t.Errorf("histogram sizes: %d %d", len(splitCSV(d["hours_utc"])), len(splitCSV(d["week_utc"])))
	}
	if f.Confidence <= 0 || f.Confidence > 1 {
		t.Errorf("confidence %v", f.Confidence)
	}
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

func TestFindingIsOrderIndependent(t *testing.T) {
	ts := stamps(30, 10)
	rev := make([]time.Time, len(ts))
	for i := range ts {
		rev[len(ts)-1-i] = ts[i] // oldest first, as some APIs return
	}
	a, _ := finding("X", "bob", ts)
	b, _ := finding("X", "bob", rev)
	if a.Details["first event"] != b.Details["first event"] || a.Details["last event"] != b.Details["last event"] || a.Details["last event"] != "2026-03-20" {
		t.Errorf("event range depends on input order: %v vs %v", a.Details, b.Details)
	}
}
