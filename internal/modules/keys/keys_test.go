// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package keys

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/openpgp"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestLookups(t *testing.T) {
	ls := lookups("Joe.Doe@Example.ORG")
	if len(ls) != 3 {
		t.Fatalf("%d lookups", len(ls))
	}
	if !strings.Contains(ls[0].url, "openpgpkey.example.org/.well-known/openpgpkey/example.org/hu/iy9q119eutrkn8s1mk4r39qejnbu3n5q?l=joe.doe") {
		t.Errorf("advanced WKD url: %s", ls[0].url)
	}
	if got := lookups("a@proton.me"); len(got) != 4 || !strings.Contains(got[3].url, "proton") {
		t.Errorf("proton must add its key server: %v", got)
	}
	if lookups("nope") != nil {
		t.Error("invalid address")
	}
}

func key() []byte {
	body := []byte{4, 0x5E, 0x0B, 0xE1, 0x00, 22, 0, 0}
	pkt := func(tag byte, b []byte) []byte { return append([]byte{0xC0 | tag, byte(len(b))}, b...) }
	k := pkt(6, body)
	k = append(k, pkt(13, []byte("Jane Public <jane@example.org>"))...)
	k = append(k, pkt(13, []byte("Jane P <jp@other.example>"))...)
	return k
}

func TestScanFindsKeyOnceAcrossSources(t *testing.T) {
	armored := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n" + base64.StdEncoding.EncodeToString(key()) + "\n-----END PGP PUBLIC KEY BLOCK-----\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(armored)) }))
	defer srv.Close()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	p := pool.New(4, 4)
	defer p.Close()

	// Point the lookups at the test server by calling fetch directly, as Scan would.
	k, ok := fetch(context.Background(), c, srv.URL)
	if !ok || k.Algorithm != "EdDSA" || len(k.UserIDs) != 2 {
		t.Fatalf("%v %+v", ok, k)
	}
	f := finding("jane@example.org", lookup{"test", srv.URL}, k)
	if f.Category != "pgp" || f.Details["user ids"] == "" {
		t.Errorf("%+v", f)
	}
	var names, emails int
	for _, e := range f.Entities {
		switch e.Type {
		case models.EntName:
			names++
		case models.EntEmail:
			emails++
			if e.Value != "jp@other.example" {
				t.Errorf("only the other address is a new lead, got %s", e.Value)
			}
		}
	}
	if names != 2 || emails != 1 {
		t.Errorf("entities: names=%d emails=%d", names, emails)
	}
	_ = scanner.Deps{HTTP: c, Pool: p}
	_ = openpgp.Key{}
}

func TestFetchRejectsNonKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte("<html><body>Not a key page at all, sorry!</body></html>"))
	}))
	defer srv.Close()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	for _, p := range []string{"/404", "/x"} {
		if _, ok := fetch(context.Background(), c, srv.URL+p); ok {
			t.Errorf("%s must not parse", p)
		}
	}
}
