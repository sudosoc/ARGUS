package codesearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestScan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "none@example.org" {
			_, _ = w.Write([]byte(`{"hits":{"total":0,"hits":[]}}`))
			return
		}
		if r.URL.Query().Get("q") == "err@example.org" {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"hits":{"total":{"value":12},"hits":[
		 {"repo":{"raw":"alice/tool"},"path":{"raw":"setup.py"},"content":{"snippet":"<table><tr><td>author_email=<mark>jane@example.org</mark></td></tr></table>"}},
		 {"repo":{"raw":"alice/tool"},"path":{"raw":"README.md"},"content":{"snippet":"contact <mark>Jane@Example.org</mark>"}},
		 {"repo":{"raw":"bob/other"},"path":{"raw":"x.txt"},"content":{"snippet":"something else entirely"}}]}}`))
	}))
	defer srv.Close()
	old := endpoint
	endpoint = srv.URL
	defer func() { endpoint = old }()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	run := func(email string) []models.Finding {
		var got []models.Finding
		_ = New().Scan(context.Background(), models.Target{Email: email}, scanner.Deps{HTTP: c}, func(f models.Finding) { got = append(got, f) })
		return got
	}
	got := run("jane@example.org")
	if len(got) != 1 || got[0].Status != models.StatusFound || got[0].Details["total reported"] != "12" || got[0].Details["files with the address (sample)"] != "2" {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got[0].Details["matches"], "alice/tool/setup.py — author_email=") || strings.Contains(got[0].Details["matches"], "bob/other") {
		t.Errorf("matches: %s", got[0].Details["matches"])
	}
	if len(got[0].Entities) != 1 || got[0].Entities[0].Value != "alice" || got[0].Entities[0].Strong {
		t.Errorf("entities: %+v", got[0].Entities)
	}
	if g := run("none@example.org"); len(g) != 1 || g[0].Status != models.StatusNotFound {
		t.Errorf("none: %+v", g)
	}
	if g := run("err@example.org"); len(g) != 1 || g[0].Status != models.StatusError || g[0].Details["error"] != "HTTP 503" {
		t.Errorf("error must be a coverage gap: %+v", g)
	}
}

func TestCanRun(t *testing.T) {
	s := New()
	if !s.CanRun(models.Target{Email: "a@b.org"}) || s.CanRun(models.Target{Email: "a@mailinator.com"}) || s.CanRun(models.Target{Username: "x"}) {
		t.Error("CanRun")
	}
}
