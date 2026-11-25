// Package lookalike finds domains that imitate the target's domain: typos,
// look-alike characters (including IDN homographs), bit flips and phishing
// style keywords. Only names that actually resolve are reported, with the
// addresses they point at and whether they receive mail. This is the check a
// brand or an employer runs to see who is impersonating them.
package lookalike

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sudosoc/argus/internal/intel"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

// maxCandidates bounds how many names are resolved per run.
const maxCandidates = 450

// resolveHost and hasMX are variables so tests can run offline.
var (
	resolveHost = func(ctx context.Context, h string) ([]string, error) { return net.DefaultResolver.LookupHost(ctx, h) }
	hasMX       = func(ctx context.Context, h string) bool {
		mx, err := net.DefaultResolver.LookupMX(ctx, h)
		return err == nil && len(mx) > 0
	}
)

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "lookalike" }
func (s *Scanner) Description() string {
	return "registered look-alike domains of the target domain (typos, homographs, keyword phishing names)"
}

func (s *Scanner) CanRun(t models.Target) bool {
	d, ok := intel.NormalizeDomain(t.Domain)
	return ok && !intel.IsFreeMail(d)
}

type hit struct {
	c     Candidate
	addrs []string
	mx    bool
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	domain, _ := intel.NormalizeDomain(t.Domain)
	sld, tld, ok := Split(domain)
	if !ok {
		return nil
	}
	root := sld + "." + tld
	cands := Generate(root, maxCandidates)

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		hits []hit
		done int
	)
	for _, c := range cands {
		wg.Add(1)
		err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			rctx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			addrs, err := resolveHost(rctx, c.Domain)
			mu.Lock()
			done++
			n := done
			mu.Unlock()
			if d.Progress != nil {
				d.Progress(s.Name(), n, len(cands))
			}
			if err != nil || len(addrs) == 0 {
				return
			}
			h := hit{c: c, addrs: addrs, mx: hasMX(rctx, c.Domain)}
			mu.Lock()
			hits = append(hits, h)
			mu.Unlock()
		})
		if err != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].mx != hits[j].mx {
			return hits[i].mx // mail-capable look-alikes are the dangerous ones
		}
		return hits[i].c.Domain < hits[j].c.Domain
	})
	if len(hits) == 0 {
		emit(models.Finding{Source: "Look-alike domains", Category: "lookalike", Status: models.StatusNotFound,
			Details: map[string]string{"checked": strconv.Itoa(len(cands)), "domain": root}})
		return nil
	}
	var lines []string
	var ents []models.Entity
	withMail := 0
	for _, h := range hits {
		line := h.c.Display
		if h.c.Display != h.c.Domain {
			line += " (" + h.c.Domain + ")"
		}
		line += " [" + h.c.Technique + "] → " + strings.Join(util.ClipList(h.addrs, 2), ", ")
		if h.mx {
			line += " + mail"
			withMail++
		}
		lines = append(lines, line)
		if len(ents) < 10 && h.mx {
			ents = append(ents, models.Entity{Type: models.EntDomain, Value: h.c.Domain})
		}
	}
	d1 := map[string]string{
		"domain": root, "checked": strconv.Itoa(len(cands)), "registered look-alikes": strconv.Itoa(len(hits)),
		"able to receive mail": strconv.Itoa(withMail), "look-alikes": strings.Join(util.ClipList(lines, 60), "\n"),
		"note": "a look-alike that resolves is not necessarily hostile (defensive registrations are common); check registration date, content and mail setup",
	}
	conf := 0.7
	if withMail > 0 {
		conf = 0.8
	}
	emit(models.Finding{Source: "Look-alike domains", Category: "lookalike", Status: models.StatusFound, Confidence: conf, Details: d1, Entities: ents})
	return nil
}
