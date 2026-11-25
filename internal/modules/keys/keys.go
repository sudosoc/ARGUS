// Package keys looks for the public OpenPGP key published for an e-mail
// address and reads who it belongs to: the names and other addresses on its
// user IDs, its fingerprint and when it was made. It asks the places a key is
// published on purpose: the domain's own Web Key Directory, keys.openpgp.org
// (verified addresses only) and, for Proton Mail addresses, Proton's key
// server. A key existing at all also proves the mailbox is real.
package keys

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/openpgp"
	"github.com/sudosoc/argus/internal/scanner"
	"github.com/sudosoc/argus/internal/util"
)

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "keys" }
func (s *Scanner) Description() string {
	return "public OpenPGP key of an e-mail address: names and addresses on it, fingerprint, creation date (WKD, keys.openpgp.org, Proton)"
}
func (s *Scanner) CanRun(t models.Target) bool { return strings.Contains(t.Email, "@") }

var protonDomains = map[string]bool{"proton.me": true, "protonmail.com": true, "pm.me": true, "protonmail.ch": true, "proton.ch": true}

type lookup struct {
	label string
	url   string
}

// lookups lists where to ask for email's key.
func lookups(email string) []lookup {
	local, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@")
	if !ok || local == "" || domain == "" {
		return nil
	}
	h := openpgp.WKDHash(local)
	l := url.QueryEscape(local)
	out := []lookup{
		{"Web Key Directory (advanced)", "https://openpgpkey." + domain + "/.well-known/openpgpkey/" + domain + "/hu/" + h + "?l=" + l},
		{"Web Key Directory (direct)", "https://" + domain + "/.well-known/openpgpkey/hu/" + h + "?l=" + l},
		{"keys.openpgp.org", "https://keys.openpgp.org/vks/v1/by-email/" + url.PathEscape(email)},
	}
	if protonDomains[domain] {
		out = append(out, lookup{"Proton key server", "https://mail-api.proton.me/pks/lookup?op=get&search=" + url.QueryEscape(email)})
	}
	return out
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	email := strings.ToLower(strings.TrimSpace(t.Email))
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = map[string]bool{} // fingerprint: the same key is often served by several places
	)
	for _, lk := range lookups(email) {
		wg.Add(1)
		if err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			k, ok := fetch(ctx, d.HTTP, lk.url)
			if !ok {
				return
			}
			mu.Lock()
			dup := seen[k.Fingerprint+k.Created.String()]
			seen[k.Fingerprint+k.Created.String()] = true
			mu.Unlock()
			if dup {
				return
			}
			emit(finding(email, lk, k))
		}); err != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
	return ctx.Err()
}

func fetch(ctx context.Context, c *httpx.Client, u string) (*openpgp.Key, bool) {
	resp, err := c.Do(ctx, httpx.Request{URL: u, Timeout: 20 * time.Second, MaxBody: 1 << 20,
		Headers: map[string]string{"Accept": "application/pgp-keys, */*"}})
	if err != nil || resp.StatusCode != 200 || len(resp.Body) < 20 {
		return nil, false
	}
	bin, err := openpgp.Dearmor(resp.Body)
	if err != nil {
		return nil, false
	}
	k, err := openpgp.Parse(bin)
	if err != nil {
		return nil, false
	}
	return k, true
}

func finding(email string, lk lookup, k *openpgp.Key) models.Finding {
	d := map[string]string{
		"published via": lk.label, "algorithm": k.Algorithm,
		"created":     k.Created.Format("2006-01-02"),
		"fingerprint": k.Fingerprint,
		"note":        "a published key shows the mailbox is real and who the owner says they are; user IDs are self-declared",
	}
	var ents []models.Entity
	var ids []string
	for _, uid := range k.UserIDs {
		id := openpgp.ParseUserID(uid)
		ids = append(ids, util.Clip(uid, 120))
		if id.Name != "" {
			ents = append(ents, models.Entity{Type: models.EntName, Value: id.Name})
		}
		if id.Email != "" && id.Email != email {
			ents = append(ents, models.Entity{Type: models.EntEmail, Value: id.Email})
		}
	}
	if len(ids) > 0 {
		d["user ids"] = strings.Join(util.ClipList(ids, 8), "; ")
	}
	if k.Fingerprint != "" {
		d["key page"] = "https://keys.openpgp.org/search?q=" + k.Fingerprint
	}
	conf := 0.9
	return models.Finding{Source: "OpenPGP key (" + lk.label + ")", Category: "pgp", URL: lk.url, Status: models.StatusFound, Confidence: conf, Details: d, Entities: ents}
}
