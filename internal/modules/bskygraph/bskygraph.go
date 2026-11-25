// Package bskygraph maps the people around a Bluesky account using only the
// public AT Protocol API: who follows the account back (mutuals) and which
// accounts the account's own follows tend to follow while the account does
// not. The second list is where friends, colleagues and family that the
// subject does not advertise tend to show up.
package bskygraph

import (
	"context"
	"net/url"
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

var api = "https://public.api.bsky.app/xrpc"

// sample is how many of the subject's follows are expanded to second degree.
const sample = 25

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "bskygraph" }
func (s *Scanner) Description() string {
	return "Bluesky social circle of a username: mutual follows and likely connections from second-degree follows"
}
func (s *Scanner) CanRun(t models.Target) bool { return len(t.Usernames()) > 0 }

// Account is the part of an app.bsky.actor.defs#profileView the graph needs.
type Account struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type page struct {
	Follows   []Account `json:"follows"`
	Followers []Account `json:"followers"`
	Cursor    string    `json:"cursor"`
}

func handleFor(user string) string {
	user = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(user), "@"))
	if user == "" {
		return ""
	}
	if !strings.Contains(user, ".") {
		user += ".bsky.social"
	}
	return user
}

func list(ctx context.Context, c *httpx.Client, kind, actor string) ([]Account, bool) {
	var p page
	u := api + "/app.bsky.graph." + kind + "?limit=100&actor=" + url.QueryEscape(actor)
	if !util.GetJSON(ctx, c, u, &p, util.Timeout(25*time.Second)) {
		return nil, false
	}
	if kind == "getFollowers" {
		return p.Followers, true
	}
	return p.Follows, true
}

// Rank returns the accounts followed by many of the subject's follows but not
// by the subject, most-shared first. follows are the subject's own follows;
// second maps each of those to the accounts they follow.
func Rank(subject string, follows []Account, second map[string][]Account, top int) []Ranked {
	mine := map[string]bool{subject: true}
	for _, f := range follows {
		mine[f.DID] = true
	}
	count := map[string]*Ranked{}
	for _, list := range second {
		for _, a := range list {
			if mine[a.DID] {
				continue
			}
			r := count[a.DID]
			if r == nil {
				r = &Ranked{Account: a}
				count[a.DID] = r
			}
			r.Shared++
		}
	}
	out := make([]Ranked, 0, len(count))
	for _, r := range count {
		if r.Shared >= 2 { // a single shared follow is noise
			out = append(out, *r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Shared != out[j].Shared {
			return out[i].Shared > out[j].Shared
		}
		return out[i].Handle < out[j].Handle
	})
	if len(out) > top {
		out = out[:top]
	}
	return out
}

// Ranked is a likely connection.
type Ranked struct {
	Account
	Shared int
}

// Mutuals returns accounts that follow the subject and are followed back.
func Mutuals(follows, followers []Account) []Account {
	in := map[string]bool{}
	for _, f := range followers {
		in[f.DID] = true
	}
	var out []Account
	for _, f := range follows {
		if in[f.DID] {
			out = append(out, f)
		}
	}
	return out
}

func label(a Account) string {
	if a.DisplayName != "" {
		return a.DisplayName + " (@" + a.Handle + ")"
	}
	return "@" + a.Handle
}

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	for _, user := range t.Usernames() {
		if h := handleFor(user); h != "" {
			s.one(ctx, h, d, emit)
		}
	}
	return ctx.Err()
}

func (s *Scanner) one(ctx context.Context, handle string, d scanner.Deps, emit func(models.Finding)) {
	var me Account
	if !util.GetJSON(ctx, d.HTTP, api+"/app.bsky.actor.getProfile?actor="+url.QueryEscape(handle), &me) || me.DID == "" {
		return // no such Bluesky account: not an error, most usernames do not exist there
	}
	follows, ok1 := list(ctx, d.HTTP, "getFollows", me.DID)
	followers, ok2 := list(ctx, d.HTTP, "getFollowers", me.DID)
	if !ok1 && !ok2 {
		return
	}

	second := map[string][]Account{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, f := range follows {
		if i >= sample {
			break
		}
		wg.Add(1)
		if err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			if l, ok := list(ctx, d.HTTP, "getFollows", f.DID); ok {
				mu.Lock()
				second[f.DID] = l
				mu.Unlock()
			}
		}); err != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		return
	}

	det := map[string]string{
		"account": "@" + me.Handle, "follows sampled": strconv.Itoa(len(follows)), "followers sampled": strconv.Itoa(len(followers)),
		"note": "public follow lists only; a connection is a lead about who the subject interacts with, not proof of a relationship",
	}
	var ents []models.Entity
	if m := Mutuals(follows, followers); len(m) > 0 {
		var names []string
		for _, a := range util.ClipList(m, 25) {
			names = append(names, label(a))
		}
		det["mutual follows"] = strings.Join(names, "; ")
		det["mutual count"] = strconv.Itoa(len(m))
	}
	if r := Rank(me.DID, follows, second, 15); len(r) > 0 {
		var lines []string
		for _, x := range r {
			lines = append(lines, label(x.Account)+" — followed by "+strconv.Itoa(x.Shared)+" of the "+strconv.Itoa(min(len(follows), sample))+" accounts checked")
		}
		det["likely connections"] = strings.Join(lines, "\n")
	}
	if det["mutual follows"] == "" && det["likely connections"] == "" {
		return
	}
	emit(models.Finding{Source: "Bluesky social circle", Category: "graph", URL: "https://bsky.app/profile/" + me.Handle,
		Status: models.StatusFound, Confidence: 0.6, Details: det, Entities: ents})
}
