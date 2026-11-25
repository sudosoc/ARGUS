// Package sanctions screens a person's or company's name against the public
// sanctions lists that can be downloaded without an account: the US OFAC
// Specially Designated Nationals list and the UN Security Council
// consolidated list. A name match is a lead, never a finding of fact: many
// people share a name, and listed people use aliases.
package sanctions

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

// Source URLs; tests replace them.
var (
	ofacSDN = []string{"https://www.treasury.gov/ofac/downloads/sdn.csv", "https://sanctionslistservice.ofac.treas.gov/api/PublicationPreview/exports/SDN.CSV"}
	ofacAlt = []string{"https://www.treasury.gov/ofac/downloads/alt.csv", "https://sanctionslistservice.ofac.treas.gov/api/PublicationPreview/exports/ALT.CSV"}
	unList  = []string{"https://scsanctions.un.org/resources/xml/en/consolidated.xml"}
)

const cacheTTL = 12 * time.Hour

// Scanner is the module.
type Scanner struct {
	mu     sync.Mutex
	loaded time.Time
	ix     *index
	counts map[string]int
	errs   []string
}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "sanctions" }
func (s *Scanner) Description() string {
	return "screens a name or company against the OFAC SDN and UN Security Council sanctions lists (leads, not proof)"
}
func (s *Scanner) CanRun(t models.Target) bool {
	return len(strings.Fields(t.FullName)) >= 2 || strings.TrimSpace(t.Company) != ""
}

func fetchFirst(ctx context.Context, c *httpx.Client, urls []string) ([]byte, string, error) {
	var last error
	for _, u := range urls {
		resp, err := c.Do(ctx, httpx.Request{URL: u, Timeout: 90 * time.Second, MaxBody: 40 << 20})
		if err != nil {
			last = err
			continue
		}
		if resp.StatusCode != 200 || len(resp.Body) < 100 {
			last = fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
			continue
		}
		return resp.Body, u, nil
	}
	return nil, "", last
}

// load downloads the lists once and keeps them for cacheTTL (a web session
// runs many scans; the lists change a few times a week).
func (s *Scanner) load(ctx context.Context, c *httpx.Client) (*index, map[string]int, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ix != nil && time.Since(s.loaded) < cacheTTL {
		return s.ix, s.counts, s.errs
	}
	var all []Entry
	counts := map[string]int{}
	var errs []string
	if sdn, _, err := fetchFirst(ctx, c, ofacSDN); err != nil {
		errs = append(errs, "OFAC SDN: "+err.Error())
	} else {
		var alt *bytes.Reader
		if b, _, err := fetchFirst(ctx, c, ofacAlt); err == nil {
			alt = bytes.NewReader(b)
		}
		var e []Entry
		if alt != nil {
			e = parseOFAC(bytes.NewReader(sdn), alt)
		} else {
			e = parseOFAC(bytes.NewReader(sdn), nil)
		}
		counts["OFAC SDN"] = len(e)
		all = append(all, e...)
	}
	if b, _, err := fetchFirst(ctx, c, unList); err != nil {
		errs = append(errs, "UN Security Council: "+err.Error())
	} else {
		e := parseUN(bytes.NewReader(b))
		counts["UN Security Council"] = len(e)
		all = append(all, e...)
	}
	if len(all) == 0 {
		return nil, counts, errs
	}
	s.ix, s.counts, s.errs, s.loaded = buildIndex(all), counts, errs, time.Now()
	return s.ix, s.counts, s.errs
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	ix, counts, errs := s.load(ctx, d.HTTP)
	for _, e := range errs {
		emit(models.Finding{Source: "Sanctions lists", Category: "sanctions", Status: models.StatusError, Details: map[string]string{"error": e}})
	}
	if ix == nil {
		return ctx.Err()
	}
	var checked []string
	for _, l := range util.SortedKeys(counts) {
		checked = append(checked, l+" ("+strconv.Itoa(counts[l])+" names)")
	}
	run := func(kind, name string, match func(string) []Entry) {
		hits := match(name)
		if len(hits) == 0 {
			emit(models.Finding{Source: "Sanctions screening: " + name, Category: "sanctions", Status: models.StatusNotFound,
				Details: map[string]string{"subject": name, "lists checked": strings.Join(checked, "; "), "result": "no name match"}})
			return
		}
		emit(hitFinding(kind, name, hits, checked))
	}
	if len(strings.Fields(t.FullName)) >= 2 {
		run("person", t.FullName, ix.matchPerson)
	}
	if strings.TrimSpace(t.Company) != "" {
		run("company", t.Company, ix.matchOrg)
	}
	return nil
}

func hitFinding(kind, name string, hits []Entry, checked []string) models.Finding {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Alias != hits[j].Alias {
			return !hits[i].Alias
		}
		return hits[i].List+hits[i].ID < hits[j].List+hits[j].ID
	})
	var lines []string
	seen := map[string]bool{}
	for _, h := range hits {
		key := h.List + h.ID
		if seen[key] || len(lines) >= 8 {
			continue
		}
		seen[key] = true
		how := "listed name"
		if h.Alias {
			how = "alias"
		}
		line := fmt.Sprintf("%s #%s: %q (%s, %s", h.List, h.ID, h.Name, how, h.Kind)
		if h.Programs != "" {
			line += "; " + util.Clip(h.Programs, 80)
		}
		line += ")"
		if h.Remarks != "" {
			line += " " + util.Clip(h.Remarks, 220)
		}
		lines = append(lines, line)
	}
	det := map[string]string{
		"subject": name, "kind": kind, "matches": strconv.Itoa(len(seen)), "listed entries": strings.Join(lines, "\n"),
		"lists checked": strings.Join(checked, "; "),
		"important":     "name match only. Compare date of birth, nationality, address and the listed programme with what you know about the subject before drawing any conclusion",
	}
	return models.Finding{Source: "Sanctions screening: " + name, Category: "sanctions", Status: models.StatusUncertain, Confidence: 0.5, Details: det}
}
