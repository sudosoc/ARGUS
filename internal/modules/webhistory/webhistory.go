// Package webhistory reads what a site used to say about its owner. It picks
// archived copies of the home page across the years (Internet Archive) and
// extracts the analytics, advertising and pixel IDs embedded in each. An ID
// that appears on an old site and a new one ties them to one operator; an ID
// that vanishes or changes marks a change of owner or agency.
package webhistory

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sudosoc/argus/internal/extract"
	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/intel"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

// base is the Internet Archive; tests point it at a local server.
var base = "https://web.archive.org"

// maxSnapshots bounds how many archived copies are downloaded.
const maxSnapshots = 10

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "webhistory" }
func (s *Scanner) Description() string {
	return "analytics / ad / pixel IDs of a site across its archived history (links old and new sites to one operator)"
}
func (s *Scanner) CanRun(t models.Target) bool {
	d, ok := intel.NormalizeDomain(t.Domain)
	return ok && !intel.IsFreeMail(d)
}

// parseCDX returns the snapshot timestamps from a CDX JSON answer
// (header row first, then rows whose first column is the timestamp).
func parseCDX(body []byte) []string {
	var rows [][]string
	if json.Unmarshal(body, &rows) != nil || len(rows) < 2 {
		return nil
	}
	var out []string
	for _, r := range rows[1:] {
		if len(r) > 0 && len(r[0]) >= 8 {
			out = append(out, r[0])
		}
	}
	return out
}

// pick chooses up to max timestamps spread over the years: the first and last
// capture plus, for every other year with captures, its earliest one.
func pick(ts []string, max int) []string {
	if len(ts) == 0 {
		return nil
	}
	sort.Strings(ts)
	byYear := map[string]string{}
	var years []string
	for _, t := range ts {
		y := t[:4]
		if _, ok := byYear[y]; !ok {
			byYear[y] = t
			years = append(years, y)
		}
	}
	chosen := []string{}
	if len(years) <= max {
		for _, y := range years {
			chosen = append(chosen, byYear[y])
		}
	} else { // thin out evenly, always keeping the first year
		for i := 0; i < max; i++ {
			chosen = append(chosen, byYear[years[i*(len(years)-1)/(max-1)]])
		}
	}
	if last := ts[len(ts)-1]; chosen[len(chosen)-1][:4] != last[:4] {
		chosen = append(chosen, last)
	}
	return chosen
}

// idSpan is where an ID was seen.
type idSpan struct {
	vendor, id  string
	first, last string // timestamps
	count       int
}

// spans folds per-snapshot extractions into first/last sightings.
func spans(shots map[string]map[string][]string) []idSpan {
	idx := map[string]*idSpan{}
	var ts []string
	for t := range shots {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	for _, t := range ts {
		for vendor, ids := range shots[t] {
			for _, id := range ids {
				k := vendor + "|" + id
				sp := idx[k]
				if sp == nil {
					sp = &idSpan{vendor: vendor, id: id, first: t}
					idx[k] = sp
				}
				sp.last = t
				sp.count++
			}
		}
	}
	out := make([]idSpan, 0, len(idx))
	for _, sp := range idx {
		out = append(out, *sp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].first != out[j].first {
			return out[i].first < out[j].first
		}
		return out[i].vendor+out[i].id < out[j].vendor+out[j].id
	})
	return out
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	domain, _ := intel.NormalizeDomain(t.Domain)
	resp, err := d.HTTP.Do(ctx, httpx.Request{
		URL:     base + "/cdx/search/cdx?url=" + domain + "&output=json&fl=timestamp,statuscode&filter=statuscode:200&collapse=timestamp:6&limit=600",
		Timeout: 60 * time.Second, MaxBody: 4 << 20,
	})
	if err != nil {
		if ctx.Err() == nil {
			emit(models.Finding{Source: "Internet Archive (site history)", Category: "tracking", Status: models.StatusError, Details: map[string]string{"error": err.Error()}})
		}
		return nil
	}
	if resp.StatusCode != 200 {
		emit(models.Finding{Source: "Internet Archive (site history)", Category: "tracking", Status: models.StatusError,
			Details: map[string]string{"error": "HTTP " + strconv.Itoa(resp.StatusCode)}})
		return nil
	}
	all := parseCDX(resp.Body)
	chosen := pick(all, maxSnapshots)
	if len(chosen) == 0 {
		return nil
	}

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		shots = map[string]map[string][]string{}
		fail  int
	)
	for _, ts := range chosen {
		wg.Add(1)
		if err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			r, err := d.HTTP.Do(ctx, httpx.Request{URL: base + "/web/" + ts + "id_/http://" + domain + "/", Timeout: 45 * time.Second, MaxBody: 2 << 20})
			mu.Lock()
			defer mu.Unlock()
			if err != nil || r.StatusCode != 200 {
				fail++
				return
			}
			shots[ts] = extract.TrackingIDs(string(r.Body))
		}); err != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	sp := spans(shots)
	det := map[string]string{
		"domain": domain, "archived captures": strconv.Itoa(len(all)), "captures read": strconv.Itoa(len(shots)),
		"first capture": fmtTS(chosen[0]), "latest capture": fmtTS(chosen[len(chosen)-1]),
	}
	if fail > 0 {
		det["captures that could not be read"] = strconv.Itoa(fail)
	}
	if len(sp) == 0 {
		det["result"] = "no analytics, advertising or pixel IDs found in the sampled captures"
		emit(models.Finding{Source: "Tracking-ID history", Category: "tracking", URL: base + "/web/*/" + domain, Status: models.StatusNotFound, Details: det})
		return nil
	}
	var lines []string
	var changed []string
	lastShot := chosen[len(chosen)-1]
	for _, x := range sp {
		span := fmtTS(x.first)[:4]
		if x.last[:4] != x.first[:4] {
			span += " – " + x.last[:4]
		}
		lines = append(lines, x.vendor+" "+x.id+": "+span)
		if x.last != lastShot && len(shots) > 1 {
			changed = append(changed, x.vendor+" "+x.id+" (gone after "+x.last[:4]+")")
		}
	}
	det["IDs by period seen"] = strings.Join(util.ClipList(lines, 40), "\n")
	if len(changed) > 0 {
		det["no longer present in the latest capture"] = strings.Join(util.ClipList(changed, 20), "; ")
		det["interpretation"] = "an ID that disappears while the site stays up often marks a change of owner, agency or analytics setup; the same ID on another site links the two"
	}
	det["pivot"] = "search each ID on a source-code search such as publicwww.com to find sibling sites"
	emit(models.Finding{Source: "Tracking-ID history", Category: "tracking", URL: base + "/web/*/" + domain, Status: models.StatusFound, Confidence: 0.8, Details: det})
	return nil
}

func fmtTS(ts string) string {
	if len(ts) >= 8 {
		return ts[:4] + "-" + ts[4:6] + "-" + ts[6:8]
	}
	return ts
}
