// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/app"
	"github.com/sudosoc/argus/internal/media"
	"github.com/sudosoc/argus/internal/models"
)

const testToken = "0123456789abcdef0123456789abcdef"

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts, _ := newTestServerWith(t)
	return ts
}

func newTestServerWith(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	base := app.Defaults()
	base.DorkEngines = []string{"duckduckgo"}
	base.DorkDelay = 10 * time.Millisecond
	base.Timeout = 3 * time.Second
	base.Retries = 0
	s, err := New(ctx, Config{Addr: "127.0.0.1:0", Token: testToken, Base: base, SettingsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func do(t *testing.T, method, url, body string, hdr map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func authed() map[string]string {
	return map[string]string{"Cookie": cookieName + "=" + testToken, "X-Requested-With": "argus", "Content-Type": "application/json"}
}

func TestAuthAndCSRF(t *testing.T) {
	ts := newTestServer(t)

	if r := do(t, "GET", ts.URL+"/api/info", "", nil); r.StatusCode != 401 {
		t.Fatalf("no cookie: got %d", r.StatusCode)
	}
	if r := do(t, "GET", ts.URL+"/?token=wrong", "", nil); r.StatusCode != 401 {
		t.Fatalf("bad token: got %d", r.StatusCode)
	}
	r := do(t, "GET", ts.URL+"/?token="+testToken, "", nil)
	if r.StatusCode != 303 || !strings.Contains(r.Header.Get("Set-Cookie"), "HttpOnly") ||
		!strings.Contains(r.Header.Get("Set-Cookie"), "SameSite=Strict") {
		t.Fatalf("token exchange: %d %q", r.StatusCode, r.Header.Get("Set-Cookie"))
	}
	if r := do(t, "GET", ts.URL+"/api/info", "", authed()); r.StatusCode != 200 {
		t.Fatalf("authed info: %d", r.StatusCode)
	}
	// DNS rebinding: valid cookie but foreign Host.
	h := authed()
	req, _ := http.NewRequest("GET", ts.URL+"/api/info", nil)
	req.Host = "evil.example"
	req.Header.Set("Cookie", h["Cookie"])
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 403 {
		t.Fatalf("foreign host: %d", resp.StatusCode)
	}
	// CSRF: missing custom header, and cross-origin.
	noHdr := map[string]string{"Cookie": h["Cookie"]}
	if r := do(t, "POST", ts.URL+"/api/scans", `{}`, noHdr); r.StatusCode != 403 {
		t.Fatalf("missing header: %d", r.StatusCode)
	}
	h["Origin"] = "http://evil.example"
	if r := do(t, "POST", ts.URL+"/api/scans", `{}`, h); r.StatusCode != 403 {
		t.Fatalf("cross origin: %d", r.StatusCode)
	}
}

func TestValidation(t *testing.T) {
	ts := newTestServer(t)
	for body, want := range map[string]int{
		`{}`:                                  400, // no identifiers
		`{"username":"x","modules":["nope"]}`: 400, // unknown module
		`{"username":"x","bogus":1}`:          400, // unknown field
		`not json`:                            400,
	} {
		if r := do(t, "POST", ts.URL+"/api/scans", body, authed()); r.StatusCode != want {
			t.Errorf("%s: got %d want %d", body, r.StatusCode, want)
		}
	}
	if r := do(t, "GET", ts.URL+"/api/scans/doesnotexist", "", authed()); r.StatusCode != 404 {
		t.Errorf("missing scan: %d", r.StatusCode)
	}
}

func TestScanLifecycleOverSSE(t *testing.T) {
	ts := newTestServer(t)

	r := do(t, "POST", ts.URL+"/api/scans", `{"phone":"+15551234567","modules":["phone"]}`, authed())
	if r.StatusCode != 202 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("start: %d %s", r.StatusCode, b)
	}
	var id string
	b, _ := io.ReadAll(r.Body)
	if i := strings.Index(string(b), `"id":"`); i >= 0 {
		id = strings.SplitN(string(b)[i+6:], `"`, 2)[0]
	}
	if id == "" {
		t.Fatalf("no id in %s", b)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/scans/"+id+"/events", nil)
	req.Header.Set("Cookie", cookieName+"="+testToken)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}

	gotFinding, cancelled := false, false
	var lastEvent string
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "event: ") {
			lastEvent = strings.TrimPrefix(line, "event: ")
			if lastEvent == "finding" {
				gotFinding = true
				if !cancelled {
					cancelled = true
					do(t, "POST", ts.URL+"/api/scans/"+id+"/cancel", `{}`, authed())
				}
			}
			if lastEvent == "done" {
				break
			}
		}
	}
	if !gotFinding {
		t.Fatal("never received a finding event (the phone module is offline and must always emit)")
	}
	if lastEvent != "done" {
		t.Fatalf("stream ended without done event (last=%q, err=%v)", lastEvent, sc.Err())
	}

	exp := do(t, "GET", ts.URL+"/api/scans/"+id+"/export?format=json", "", authed())
	body, _ := io.ReadAll(exp.Body)
	if exp.StatusCode != 200 || !strings.Contains(string(body), `"phone": "+15551234567"`) {
		t.Fatalf("export: %d %s", exp.StatusCode, body)
	}
	if r := do(t, "GET", ts.URL+"/api/scans/"+id+"/export?format=exe", "", authed()); r.StatusCode != 400 {
		t.Fatalf("bad format: %d", r.StatusCode)
	}
}

func TestImageProxyServesOnlyDiscoveredPictures(t *testing.T) {
	media.AllowPrivateHostsForTests(true)
	defer media.AllowPrivateHostsForTests(false)

	pic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))
		for i := range img.Pix {
			img.Pix[i] = uint8(i)
		}
		_ = png.Encode(w, img)
	}))
	defer pic.Close()

	ts, srv := newTestServerWith(t)
	target := pic.URL + "/me.png"

	// not discovered by any scan -> refused (the proxy is not an open relay)
	if r := do(t, "GET", ts.URL+"/api/img?u="+url.QueryEscape(target), "", authed()); r.StatusCode != 403 {
		t.Fatalf("undiscovered image: %d", r.StatusCode)
	}
	if r := do(t, "GET", ts.URL+"/api/img?u="+url.QueryEscape(target), "", nil); r.StatusCode != 401 {
		t.Fatalf("unauthenticated: %d", r.StatusCode)
	}
	srv.mgr.addImage(target)
	r := do(t, "GET", ts.URL+"/api/img?u="+url.QueryEscape(target), "", authed())
	body, _ := io.ReadAll(r.Body)
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/png" || len(body) < 50 ||
		!strings.Contains(r.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("known image: %d %q len=%d", r.StatusCode, r.Header.Get("Content-Type"), len(body))
	}
}

func TestPlanProposesTheSmallestSet(t *testing.T) {
	ts := newTestServer(t)
	body := `{"username":"torvalds","email":"a@example.org","goal":"identify"}`
	resp := do(t, "POST", ts.URL+"/api/plan", body, authed())
	var out struct {
		Goal       string `json:"goal"`
		Supplied   []string
		Collectors []struct {
			Name     string
			Selected bool
			Uses     []string
			SendsTo  string `json:"sends_to"`
		}
		Never []string
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Goal != "identify" || len(out.Never) != 4 || len(out.Supplied) < 2 {
		t.Fatalf("unexpected plan: %+v", out)
	}
	sel := map[string]bool{}
	for _, c := range out.Collectors {
		sel[c.Name] = c.Selected
		if c.Selected && len(c.Uses) == 0 && c.Name != "dorks" && c.Name != "context" {
			t.Errorf("%s is proposed but reads none of the supplied details", c.Name)
		}
	}
	if !sel["social"] || !sel["profiles"] {
		t.Errorf("identify should propose social and profiles: %v", sel)
	}
	if v, ok := sel["dorks"]; ok && v {
		t.Errorf("dorks is not part of identify")
	}
}

func TestFailedPicturesAreNotRetriedAtOnce(t *testing.T) {
	var f failures
	if f.recent("u") {
		t.Fatal("nothing has failed yet")
	}
	f.mark("u")
	if !f.recent("u") || f.recent("other") {
		t.Fatal("a failed picture must be remembered, and only that one")
	}
}

func TestPlanListsOptInCollectorsButNeverProposesThem(t *testing.T) {
	ts := newTestServer(t)
	resp := do(t, "POST", ts.URL+"/api/plan", `{"username":"ann","goal":"everything"}`, authed())
	var out struct {
		Collectors []struct {
			Name     string
			Selected bool
			Optional bool
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, c := range out.Collectors {
		if strings.HasPrefix(c.Name, "posts") {
			seen++
			if !c.Optional || c.Selected {
				t.Errorf("%s must be listed as optional and not proposed: %+v", c.Name, c)
			}
		}
	}
	if seen != 4 {
		t.Fatalf("expected the four posts collectors in the plan, saw %d", seen)
	}
}

func TestCompareEndpointOnASavedCase(t *testing.T) {
	ts, srv := newTestServerWith(t)
	rep := &models.Report{Target: models.Target{Username: "x"}, Identities: []models.Identity{
		{ID: 1, Label: "A", Names: []models.Fact{{Value: "Seif Alsayed"}}, Usernames: []models.Fact{{Value: "github:programmingman1"}}, Photos: []string{"pa"}},
		{ID: 2, Label: "B", Names: []models.Fact{{Value: "sudosoc, Seif"}}, Usernames: []models.Fact{{Value: "github:sudosoc"}}, Photos: []string{"pb"}},
	}, Photos: []models.Photo{{URL: "pa", DHash: "ffff0000ffff0000"}, {URL: "pb", DHash: "ffff0000ffff01ff"}}}
	sc := srv.mgr.restore(rep)
	resp := do(t, "GET", ts.URL+"/api/scans/"+sc.ID+"/compare?a=1&b=2", "", authed())
	var c models.Comparison
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil || resp.StatusCode != 200 {
		t.Fatalf("status %d err %v", resp.StatusCode, err)
	}
	if c.A != 1 || c.B != 2 || len(c.Signals) == 0 || len(c.Photos) != 1 || c.Verdict == "unlikely" {
		t.Errorf("%+v", c)
	}
	if r := do(t, "GET", ts.URL+"/api/scans/"+sc.ID+"/compare?a=1&b=1", "", authed()); r.StatusCode != 400 {
		t.Errorf("same person twice: %d", r.StatusCode)
	}
	if r := do(t, "GET", ts.URL+"/api/scans/"+sc.ID+"/compare?a=1&b=9", "", authed()); r.StatusCode != 404 {
		t.Errorf("unknown person: %d", r.StatusCode)
	}
}
