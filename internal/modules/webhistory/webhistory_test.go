// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package webhistory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestParseCDXAndPick(t *testing.T) {
	ts := parseCDX([]byte(`[["timestamp","statuscode"],["20120105120000","200"],["20120601000000","200"],["20150101000000","200"],["20190909000000","200"],["20240101000000","200"],["bad"]]`))
	if len(ts) != 5 {
		t.Fatalf("%v", ts)
	}
	got := pick(ts, 10)
	want := []string{"20120105120000", "20150101000000", "20190909000000", "20240101000000"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("pick = %v", got)
	}
	// thinning keeps the first year and the last capture
	var many []string
	for y := 2000; y < 2024; y++ {
		many = append(many, itoa(y)+"0101000000")
	}
	p := pick(many, 5)
	if len(p) < 5 || p[0][:4] != "2000" || p[len(p)-1][:4] != "2023" {
		t.Errorf("thin pick: %v", p)
	}
	if parseCDX([]byte("garbage")) != nil || pick(nil, 3) != nil {
		t.Error("empty inputs")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestSpans(t *testing.T) {
	sp := spans(map[string]map[string][]string{
		"20120101000000": {"google-analytics": {"UA-111-1"}},
		"20160101000000": {"google-analytics": {"UA-111-1", "UA-222-1"}},
		"20230101000000": {"google-analytics": {"UA-222-1"}, "facebook-pixel": {"1234567890"}},
	})
	if len(sp) != 3 || sp[0].id != "UA-111-1" || sp[0].last != "20160101000000" || sp[1].id != "UA-222-1" || sp[1].count != 2 {
		t.Errorf("%+v", sp)
	}
}

func TestScanEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/cdx/"):
			_, _ = w.Write([]byte(`[["timestamp","statuscode"],["20120101000000","200"],["20230101000000","200"]]`))
		case strings.Contains(r.URL.Path, "20120101000000"):
			_, _ = w.Write([]byte(`<script>ga('create','UA-1234567-1')</script>`))
		default:
			_, _ = w.Write([]byte(`<script>gtag('config','G-ABCDEF1234')</script>`))
		}
	}))
	defer srv.Close()
	old := base
	base = srv.URL
	defer func() { base = old }()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	p := pool.New(4, 4)
	defer p.Close()
	var got []models.Finding
	if err := New().Scan(context.Background(), models.Target{Domain: "example.com"}, scanner.Deps{HTTP: c, Pool: p}, func(f models.Finding) { got = append(got, f) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != models.StatusFound {
		t.Fatalf("%+v", got)
	}
	d := got[0].Details
	if !strings.Contains(d["IDs by period seen"], "UA-1234567-1: 2012") || !strings.Contains(d["IDs by period seen"], "G-ABCDEF1234") {
		t.Errorf("%v", d)
	}
	if !strings.Contains(d["no longer present in the latest capture"], "UA-1234567-1") {
		t.Errorf("the vanished ID must be flagged: %v", d)
	}
}
