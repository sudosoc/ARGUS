// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

const sameplaceKm = 50.0

type sighting struct {
	lat, lon float64
	t        time.Time
	dated    bool
	label    string
	source   string
	country  string
}

func parseLatLon(s string) (float64, float64, bool) {
	a, b, ok := strings.Cut(s, ",")
	if !ok {
		return 0, 0, false
	}
	la, e1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
	lo, e2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if e1 != nil || e2 != nil || la < -90 || la > 90 || lo < -180 || lo > 180 {
		return 0, 0, false
	}
	return la, lo, true
}

// movement reads where the subject has been from geotagged, dated pictures and from the location evidence,
// groups sightings within 50 km into places, and lists the moves between places in date order.
func movement(c *ctx) *models.Movement {
	var s []sighting
	for _, p := range c.rep.Photos {
		la, lo, ok := parseLatLon(p.GPS)
		if !ok {
			continue
		}
		t, dated := parseDate(p.Taken)
		s = append(s, sighting{la, lo, t, dated, "", "picture " + strings.Join(p.Sources, ", "), ""})
	}
	if g := c.rep.Geo; g != nil {
		for _, pt := range g.Points {
			if pt.Kind == "given" || (pt.Lat == 0 && pt.Lon == 0) {
				continue
			}
			s = append(s, sighting{pt.Lat, pt.Lon, time.Time{}, false, firstNonEmpty(pt.Place, pt.Label), pt.Kind + ": " + strings.Join(pt.Sources, ", "), pt.Country})
		}
	}
	if len(s) == 0 {
		return nil
	}
	m := &models.Movement{}
	// places: greedy clustering by distance
	type cl struct {
		lat, lon float64
		items    []sighting
	}
	var cls []*cl
	assign := make([]int, len(s))
	for i, x := range s {
		best := -1
		for k, k2 := range cls {
			if haversineKm(x.lat, x.lon, k2.lat, k2.lon) <= sameplaceKm {
				best = k
				break
			}
		}
		if best < 0 {
			cls = append(cls, &cl{lat: x.lat, lon: x.lon})
			best = len(cls) - 1
		}
		cls[best].items = append(cls[best].items, x)
		assign[i] = best
	}
	names := make([]string, len(cls))
	for k, k2 := range cls {
		lbl := ""
		for _, it := range k2.items {
			if it.label != "" {
				lbl = it.label
				break
			}
		}
		if lbl == "" {
			lbl = fmt.Sprintf("%.2f, %.2f", k2.lat, k2.lon)
		}
		names[k] = lbl
		pl := models.Place{Label: lbl, Lat: k2.lat, Lon: k2.lon, N: len(k2.items)}
		for _, it := range k2.items {
			pl.Sources = append(pl.Sources, it.source)
			if it.dated {
				d := dayString(it.t)
				if pl.First == "" || d < pl.First {
					pl.First = d
				}
				if d > pl.Last {
					pl.Last = d
				}
			}
		}
		pl.Sources = uniqStrings(pl.Sources)
		m.Places = append(m.Places, pl)
	}
	sort.SliceStable(m.Places, func(i, j int) bool { return m.Places[i].N > m.Places[j].N })
	if len(m.Places) > 0 {
		m.Home = m.Places[0].Label
	}
	if g := c.rep.Geo; g != nil && g.Consensus != "" {
		m.Home = g.Consensus
	}
	// trips between dated sightings
	type dpt struct {
		t time.Time
		k int
	}
	var dated []dpt
	for i, x := range s {
		if x.dated {
			dated = append(dated, dpt{x.t, assign[i]})
		}
	}
	sort.SliceStable(dated, func(i, j int) bool { return dated[i].t.Before(dated[j].t) })
	for i := 1; i < len(dated); i++ {
		a, b := dated[i-1], dated[i]
		if a.k != b.k {
			km := haversineKm(cls[a.k].lat, cls[a.k].lon, cls[b.k].lat, cls[b.k].lon)
			if km >= sameplaceKm {
				m.Trips = append(m.Trips, models.Trip{From: names[a.k], To: names[b.k], Date: dayString(b.t), Km: float64(int(km))})
			}
		}
	}
	seenC := map[string]bool{}
	for _, x := range s {
		if x.country != "" && !seenC[x.country] {
			seenC[x.country] = true
			m.Countries = append(m.Countries, x.country)
		}
	}
	sort.Strings(m.Countries)
	if len(dated) == 0 {
		m.Notes = append(m.Notes, "No sighting carries a date, so no trips can be derived: places come from stated locations and undated GPS.")
	} else if len(dated) < 3 {
		m.Notes = append(m.Notes, "Fewer than three dated sightings: the trips shown are thin evidence.")
	}
	m.Notes = append(m.Notes, "A place is where evidence points, not where the person is now. GPS in a picture can be copied from elsewhere.")
	return m
}
