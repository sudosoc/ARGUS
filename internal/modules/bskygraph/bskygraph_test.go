package bskygraph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func acc(did, handle string) Account { return Account{DID: did, Handle: handle} }

func TestRankAndMutuals(t *testing.T) {
	follows := []Account{acc("a", "a.x"), acc("b", "b.x"), acc("c", "c.x")}
	second := map[string][]Account{
		"a": {acc("z", "z.x"), acc("y", "y.x"), acc("me", "me.x"), acc("b", "b.x")},
		"b": {acc("z", "z.x"), acc("q", "q.x")},
		"c": {acc("z", "z.x"), acc("y", "y.x")},
	}
	r := Rank("me", follows, second, 10)
	if len(r) != 2 || r[0].DID != "z" || r[0].Shared != 3 || r[1].DID != "y" || r[1].Shared != 2 {
		t.Errorf("rank: %+v", r)
	}
	m := Mutuals(follows, []Account{acc("b", "b.x"), acc("c", "c.x"), acc("d", "d.x")})
	if len(m) != 2 || m[0].DID != "b" {
		t.Errorf("mutuals: %+v", m)
	}
}

func TestHandleFor(t *testing.T) {
	for in, want := range map[string]string{"Alice": "alice.bsky.social", "@bob.example.com": "bob.example.com", " ": ""} {
		if got := handleFor(in); got != want {
			t.Errorf("%q -> %q", in, got)
		}
	}
}

func TestScan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := r.URL.Query().Get("actor")
		enc := json.NewEncoder(w)
		switch {
		case strings.HasSuffix(r.URL.Path, "getProfile"):
			if actor == "alice.bsky.social" {
				_ = enc.Encode(Account{DID: "did:me", Handle: "alice.bsky.social"})
				return
			}
			w.WriteHeader(400)
		case strings.HasSuffix(r.URL.Path, "getFollowers"):
			_ = enc.Encode(page{Followers: []Account{acc("did:f1", "f1.x")}})
		case strings.HasSuffix(r.URL.Path, "getFollows") && actor == "did:me":
			_ = enc.Encode(page{Follows: []Account{acc("did:f1", "f1.x"), acc("did:f2", "f2.x")}})
		default: // second degree: both follows follow the same two outsiders
			_ = enc.Encode(page{Follows: []Account{acc("did:o1", "o1.x"), acc("did:o2", "o2.x")}})
		}
	}))
	defer srv.Close()
	old := api
	api = srv.URL
	defer func() { api = old }()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	p := pool.New(4, 4)
	defer p.Close()
	var got []models.Finding
	if err := New().Scan(context.Background(), models.Target{Username: "alice"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { got = append(got, f) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Category != "graph" {
		t.Fatalf("%+v", got)
	}
	d := got[0].Details
	if !strings.Contains(d["mutual follows"], "f1.x") || !strings.Contains(d["likely connections"], "o1.x") {
		t.Errorf("%v", d)
	}
	// an account that does not exist is silent
	got = nil
	_ = New().Scan(context.Background(), models.Target{Username: "nobody"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { got = append(got, f) })
	if len(got) != 0 {
		t.Errorf("unknown handle must emit nothing: %+v", got)
	}
}
