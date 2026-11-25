package contextual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestContextualScan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/reverse"):
			_, _ = w.Write([]byte(`{"display_name":"Tahrir Square, Cairo, Egypt","address":{"city":"Cairo","country":"Egypt"}}`))
		case strings.HasPrefix(r.URL.Path, "/w/api.php"):
			_, _ = w.Write([]byte(`{"query":{"geosearch":[{"title":"File:Tahrir 2011.jpg","dist":120.5}]}}`))
		case strings.HasPrefix(r.URL.Path, "/api/v2/doc/doc"):
			_, _ = w.Write([]byte(`{"articles":[{"url":"https://news.example/a","title":"Layla Hassan speaks","domain":"news.example"}]}`))
		case strings.HasPrefix(r.URL.Path, "/wayback/available"):
			_, _ = w.Write([]byte(`{"archived_snapshots":{"closest":{"available":true,"url":"http://web.archive.org/web/20260110/https://hassanlabs.org","timestamp":"20260110120000"}}}`))
		case strings.Contains(r.URL.Path, "onthisday"):
			_, _ = w.Write([]byte(`{"events":[{"text":"Something happened","year":1990}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	old := endpoints
	endpoints.Nominatim, endpoints.Commons, endpoints.Wikipedia, endpoints.GDELT, endpoints.Wayback, endpoints.OnThisDay = srv.URL, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL
	defer func() { endpoints = old }()

	c, err := httpx.New(httpx.Options{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	tg := models.Target{FullName: "Layla Hassan", Domain: "hassanlabs.org", Coords: "30.04440,31.23570", DateFrom: "2026-01-10", DateTo: "2026-01-12"}
	s := New()
	if !s.CanRun(tg) || s.CanRun(models.Target{FullName: "x"}) {
		t.Fatal("CanRun")
	}
	var got []models.Finding
	if err := s.Scan(context.Background(), tg, scanner.Deps{HTTP: c}, func(f models.Finding) { got = append(got, f) }); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range got {
		have[f.Category+"|"+f.Source[:min(14, len(f.Source))]] = true
		if f.Status == models.StatusError {
			t.Errorf("unexpected error finding %s: %v", f.Source, f.Details)
		}
	}
	for _, want := range []string{"geo|coordinates 30", "geo|place at 30.04", "geo-media|geotagged pict", "archive|closest archiv", "mentions|news naming La", "context|period 2026-01"} {
		if !have[want] {
			t.Errorf("missing %q in %v", want, have)
		}
	}
	var place *models.Finding
	for i := range got {
		if strings.HasPrefix(got[i].Source, "place at") {
			place = &got[i]
		}
	}
	if place == nil || len(place.Entities) == 0 || place.Entities[0].Value != "Cairo, Egypt" {
		t.Fatalf("place entity: %+v", place)
	}
}
