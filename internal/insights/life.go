// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// life orders every dated fact and marks what is unusual: bursts of new accounts, long silences,
// very recent or very old last activity.
func life(c *ctx) *models.LifeTimeline {
	type ev struct {
		t   time.Time
		raw models.TimelineEvent
	}
	var evs []ev
	for _, e := range c.rep.Timeline {
		if t, ok := parseDate(e.Date); ok && !c.protected(e.Event) {
			evs = append(evs, ev{t, e})
		}
	}
	if len(evs) == 0 {
		return nil
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].t.Before(evs[j].t) })
	l := &models.LifeTimeline{First: dayString(evs[0].t), Last: dayString(evs[len(evs)-1].t)}
	perYear := map[int]int{}
	for _, e := range evs {
		l.Events = append(l.Events, models.LifeEvent{Date: dayString(e.t), Event: e.raw.Event, Source: e.raw.Source, URL: e.raw.URL})
		perYear[e.t.Year()]++
	}
	var years []int
	for y := range perYear {
		years = append(years, y)
	}
	sort.Ints(years)
	for _, y := range years {
		l.PerYear = append(l.PerYear, models.YearCount{Year: y, N: perYear[y]})
	}

	// bursts: three or more account creations from different sources inside 30 days
	var created []ev
	for _, e := range evs {
		if isCreation(e.raw.Event) {
			created = append(created, e)
		}
	}
	for i := 0; i < len(created); {
		j := i
		srcs := map[string]bool{created[i].raw.Source: true}
		for j+1 < len(created) && created[j+1].t.Sub(created[i].t) <= 30*24*time.Hour {
			j++
			srcs[created[j].raw.Source] = true
		}
		if len(srcs) >= 3 {
			l.Anomalies = append(l.Anomalies, models.Anomaly{Kind: "burst", Period: dayString(created[i].t) + " to " + dayString(created[j].t),
				Text: fmt.Sprintf("%d accounts were created within %d days: a launch, a rebrand or a batch of fresh identities.", len(srcs), int(created[j].t.Sub(created[i].t).Hours()/24)+1)})
			i = j + 1
			continue
		}
		i++
	}

	// gaps: two years or more without any dated trace, inside the span
	if len(evs) >= 3 {
		for i := 1; i < len(evs); i++ {
			if d := evs[i].t.Sub(evs[i-1].t); d >= 730*24*time.Hour {
				l.Anomalies = append(l.Anomalies, models.Anomaly{Kind: "gap", Period: dayString(evs[i-1].t) + " to " + dayString(evs[i].t),
					Text: fmt.Sprintf("No dated trace for %.1f years. The person may have been offline, used other accounts, or the evidence for these years was not collected.", d.Hours()/24/365.25)})
			}
		}
	}

	// recency
	last := evs[len(evs)-1].t
	switch age := time.Since(last); {
	case age <= 60*24*time.Hour:
		l.Anomalies = append(l.Anomalies, models.Anomaly{Kind: "recent", Period: dayString(last), Text: "Activity in the last two months: the subject is currently active online."})
	case age >= 730*24*time.Hour:
		l.Anomalies = append(l.Anomalies, models.Anomaly{Kind: "gap", Period: "since " + dayString(last),
			Text: fmt.Sprintf("The most recent dated trace is %.1f years old: the accounts may be abandoned or the person may no longer use these services.", age.Hours()/24/365.25)})
	}
	if len(evs) == 1 {
		l.Anomalies = append(l.Anomalies, models.Anomaly{Kind: "single", Period: dayString(evs[0].t), Text: "Only one dated fact exists: no pattern can be read."})
	}
	return l
}
