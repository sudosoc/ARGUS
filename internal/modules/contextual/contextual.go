// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package contextual works from the circumstances of a case rather than from an identifier:
// a place (coordinates) and a period (dates). It finds public, no-account material tied to them:
// the place's name, geotagged pictures and articles nearby (Wikimedia Commons, Wikipedia), news in the
// period that names the subject (GDELT), the closest Internet Archive copy of the subject's site to the
// period, what happened on those days (Wikipedia), and ready-made search links for the places that
// need a person to open them (maps, photo and social search). It never asks a social network for people near a point.
package contextual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

// endpoints are variables so tests can point them at a local server.
var endpoints = struct{ Nominatim, Commons, Wikipedia, GDELT, Wayback, OnThisDay string }{
	Nominatim: "https://nominatim.openstreetmap.org",
	Commons:   "https://commons.wikimedia.org",
	Wikipedia: "https://en.wikipedia.org",
	GDELT:     "https://api.gdeltproject.org",
	Wayback:   "https://archive.org",
	OnThisDay: "https://api.wikimedia.org",
}

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "context" }
func (s *Scanner) Description() string {
	return "place and period of interest: nearby geotagged pictures and articles, news and archive copies in the period, search links"
}
func (s *Scanner) CanRun(t models.Target) bool { return t.Coords != "" || t.DateFrom != "" }

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	if t.Coords != "" {
		geoScan(ctx, t, d.HTTP, emit)
	}
	if t.DateFrom != "" {
		dateScan(ctx, t, d.HTTP, emit)
	}
	return ctx.Err()
}

func found(source, cat, link string, conf float64, det map[string]string) models.Finding {
	return models.Finding{Source: source, Category: cat, URL: link, Status: models.StatusFound, Confidence: conf, Details: det}
}

func getJSON(ctx context.Context, c *httpx.Client, u string, out any) error {
	if c == nil {
		return fmt.Errorf("no HTTP client")
	}
	resp, err := c.Do(ctx, httpx.Request{URL: u, Timeout: 20 * time.Second, MaxBody: 2 << 20, Headers: map[string]string{"Accept": "application/json", "User-Agent": "SUDOSOC-ARGUS/0.3 (public-data OSINT; contact: operator)"}})
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(resp.Body, out)
}

func fail(emit func(models.Finding), source string, err error) {
	emit(models.Finding{Source: source, Category: "context", Status: models.StatusError, Details: map[string]string{"error": err.Error()}})
}

// ------------------------------------------------------------------- place

func geoScan(ctx context.Context, t models.Target, c *httpx.Client, emit func(models.Finding)) {
	parts := strings.Split(t.Coords, ",")
	if len(parts) != 2 {
		return
	}
	lat, lon := parts[0], parts[1]
	q := url.QueryEscape
	ll := lat + "," + lon
	// links a person opens: no account or key is used by ARGUS
	links := []string{
		"OpenStreetMap: https://www.openstreetmap.org/?mlat=" + lat + "&mlon=" + lon + "#map=17/" + lat + "/" + lon,
		"Google Maps: https://www.google.com/maps?q=" + q(ll),
		"Bing Maps: https://www.bing.com/maps?cp=" + lat + "~" + lon + "&lvl=17",
		"Mapillary street photos: https://www.mapillary.com/app/?lat=" + lat + "&lng=" + lon + "&z=17",
		"Wikimedia Commons map: https://commons.wikimedia.org/wiki/Special:Nearby#/coord/" + lat + "," + lon,
		"Flickr photos on the map: https://www.flickr.com/map?&fLat=" + lat + "&fLon=" + lon + "&zl=16",
		"Instagram location search (needs a login): https://www.instagram.com/explore/search/keyword/?q=" + q(ll),
		"X search near the point: https://x.com/search?q=" + q("geocode:"+ll+",1km") + "&f=live",
		"Google Earth: https://earth.google.com/web/@" + lat + "," + lon + ",0a,1000d,35y,0h,0t,0r",
	}
	emit(found("coordinates "+ll, "geo", "https://www.openstreetmap.org/?mlat="+lat+"&mlon="+lon+"#map=17/"+lat+"/"+lon, 0.9,
		map[string]string{"latitude": lat, "longitude": lon, "open on a map or in a photo search": strings.Join(links, "\n")}))

	var rev struct {
		DisplayName string            `json:"display_name"`
		Address     map[string]string `json:"address"`
	}
	if err := getJSON(ctx, c, endpoints.Nominatim+"/reverse?format=jsonv2&zoom=16&lat="+lat+"&lon="+lon, &rev); err != nil {
		fail(emit, "reverse geocode (OpenStreetMap)", err)
	} else if rev.DisplayName != "" {
		f := found("place at "+ll, "geo", "", 0.8, map[string]string{"place": rev.DisplayName, "source": "OpenStreetMap Nominatim"})
		for _, k := range []string{"city", "town", "village", "suburb", "state", "country"} {
			if v := rev.Address[k]; v != "" {
				f.Details[k] = v
			}
		}
		place := firstOf(rev.Address, "city", "town", "village", "state")
		if place != "" && rev.Address["country"] != "" {
			place += ", " + rev.Address["country"]
		}
		if place != "" {
			f.Entities = append(f.Entities, models.Entity{Type: models.EntLocation, Value: place})
		}
		emit(f)
	}

	var cm struct {
		Query struct {
			Geosearch []struct {
				Title string  `json:"title"`
				Dist  float64 `json:"dist"`
			} `json:"geosearch"`
		} `json:"query"`
	}
	if err := getJSON(ctx, c, endpoints.Commons+"/w/api.php?action=query&list=geosearch&gscoord="+lat+"%7C"+lon+"&gsradius=1000&gslimit=30&gsnamespace=6&format=json", &cm); err != nil {
		fail(emit, "geotagged pictures (Wikimedia Commons)", err)
	} else if n := len(cm.Query.Geosearch); n > 0 {
		var lines []string
		for _, g := range cm.Query.Geosearch {
			lines = append(lines, fmt.Sprintf("%s (%.0f m): https://commons.wikimedia.org/wiki/%s", strings.TrimPrefix(g.Title, "File:"), g.Dist, url.PathEscape(strings.ReplaceAll(g.Title, " ", "_"))))
		}
		emit(found("geotagged pictures within 1 km", "geo-media", "", 0.6, map[string]string{"count": strconv.Itoa(n), "pictures": strings.Join(lines, "\n"), "source": "Wikimedia Commons"}))
	}

	var wp struct {
		Query struct {
			Geosearch []struct {
				Title string  `json:"title"`
				Dist  float64 `json:"dist"`
			} `json:"geosearch"`
		} `json:"query"`
	}
	if err := getJSON(ctx, c, endpoints.Wikipedia+"/w/api.php?action=query&list=geosearch&gscoord="+lat+"%7C"+lon+"&gsradius=2000&gslimit=30&format=json", &wp); err != nil {
		fail(emit, "places nearby (Wikipedia)", err)
	} else if n := len(wp.Query.Geosearch); n > 0 {
		var lines []string
		for _, g := range wp.Query.Geosearch {
			lines = append(lines, fmt.Sprintf("%s (%.0f m)", g.Title, g.Dist))
		}
		emit(found("places and landmarks within 2 km", "geo", "", 0.6, map[string]string{"count": strconv.Itoa(n), "places": strings.Join(lines, "\n"), "source": "Wikipedia"}))
	}
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if m[k] != "" {
			return m[k]
		}
	}
	return ""
}

// ------------------------------------------------------------------ period

func dateScan(ctx context.Context, t models.Target, c *httpx.Client, emit func(models.Finding)) {
	from, err1 := time.Parse("2006-01-02", t.DateFrom)
	to, err2 := time.Parse("2006-01-02", t.DateTo)
	if err1 != nil || err2 != nil {
		return
	}
	span := t.DateFrom
	if t.DateTo != t.DateFrom {
		span += " to " + t.DateTo
	}
	q := url.QueryEscape
	terms := []string{}
	for _, v := range []string{t.FullName, t.Username, t.Company} {
		if v != "" {
			terms = append(terms, v)
		}
	}
	var links []string
	for _, v := range terms {
		links = append(links,
			"Google, only the period ("+v+"): https://www.google.com/search?q="+q(`"`+v+`" after:`+t.DateFrom+` before:`+to.AddDate(0, 0, 1).Format("2006-01-02")),
			"X advanced search ("+v+"): https://x.com/search?q="+q(`"`+v+`" since:`+t.DateFrom+` until:`+to.AddDate(0, 0, 1).Format("2006-01-02"))+"&f=live",
			"Reddit ("+v+"): https://www.reddit.com/search/?q="+q(v)+"&sort=new",
		)
	}
	if t.Domain != "" {
		links = append(links, "Internet Archive calendar of "+t.Domain+": https://web.archive.org/web/"+from.Format("20060102")+"*/"+t.Domain)
	}
	if len(links) > 0 {
		emit(found("period "+span+": searches to open", "context", "", 0.5, map[string]string{"period": span, "links": strings.Join(links, "\n")}))
	}

	if t.Domain != "" {
		var raw struct {
			Archived struct {
				Closest struct {
					URL       string `json:"url"`
					Timestamp string `json:"timestamp"`
					Available bool   `json:"available"`
				} `json:"closest"`
			} `json:"archived_snapshots"`
		}
		if err := getJSON(ctx, c, endpoints.Wayback+"/wayback/available?url="+q(t.Domain)+"&timestamp="+from.Format("20060102"), &raw); err != nil {
			fail(emit, "Internet Archive copy of "+t.Domain, err)
		} else if cl := raw.Archived.Closest; cl.Available {
			emit(found("closest archived copy of "+t.Domain+" to "+t.DateFrom, "archive", cl.URL, 0.8, map[string]string{"captured": cl.Timestamp, "asked for": t.DateFrom, "url": cl.URL}))
		} else {
			emit(models.Finding{Source: "closest archived copy of " + t.Domain, Category: "archive", Status: models.StatusNotFound, Details: map[string]string{"note": "the Internet Archive holds no copy near " + t.DateFrom}})
		}
	}

	if len(terms) > 0 {
		for _, v := range terms[:1] { // the strongest identifier first; GDELT is rate limited
			var g struct {
				Articles []struct {
					URL, Title, Domain, SeenDate string
					Language, SourceCountry      string
				} `json:"articles"`
			}
			u := endpoints.GDELT + "/api/v2/doc/doc?query=" + q(`"`+v+`"`) + "&mode=ArtList&format=json&maxrecords=50&startdatetime=" + from.Format("20060102") + "000000&enddatetime=" + to.Format("20060102") + "235959"
			if err := getJSON(ctx, c, u, &g); err != nil {
				fail(emit, "news in the period (GDELT)", err)
				continue
			}
			var lines []string
			for _, a := range g.Articles {
				lines = append(lines, fmt.Sprintf("%s · %s · %s", a.Title, a.Domain, a.URL))
			}
			if len(lines) > 0 {
				emit(found("news naming "+v+" in "+span, "mentions", "", 0.4, map[string]string{"articles": strings.Join(lines, "\n"), "count": strconv.Itoa(len(lines)), "note": "headline matches only; confirm each is about the right person", "source": "GDELT"}))
			} else {
				emit(models.Finding{Source: "news naming " + v + " in " + span, Category: "mentions", Status: models.StatusNotFound, Details: map[string]string{"source": "GDELT"}})
			}
		}
	}

	var ev struct {
		Events []struct {
			Text string `json:"text"`
			Year int    `json:"year"`
		} `json:"events"`
	}
	if err := getJSON(ctx, c, endpoints.OnThisDay+fmt.Sprintf("/feed/v1/wikipedia/en/onthisday/events/%02d/%02d", int(from.Month()), from.Day()), &ev); err == nil && len(ev.Events) > 0 {
		var lines []string
		for i, e := range ev.Events {
			if i >= 12 {
				break
			}
			lines = append(lines, fmt.Sprintf("%d: %s", e.Year, e.Text))
		}
		emit(found("on this day: "+from.Format("2 January"), "context", "", 0.3, map[string]string{"events": strings.Join(lines, "\n"), "note": "general history for that calendar day, not about the subject", "source": "Wikipedia"}))
	}
}
