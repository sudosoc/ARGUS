// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Command sitesync builds ARGUS's embedded username-site list from the
// WhatsMyName, Sherlock and Maigret datasets.
//
//	go run ./cmd/sitesync -fetch                 # download the three lists, convert, write
//	go run ./cmd/sitesync -in /path/to/snapshots # wmn.json sherlock.json maigret.json already on disk
//
// The result goes to internal/modules/social/sites_default.json.gz and the
// sources (URL, SHA-256, counts) are recorded in data/sites/sources.lock.json.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sudosoc/argus/internal/modules/social"
	"github.com/sudosoc/argus/internal/sitesync"
)

type source struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	License string `json:"license"`
	SHA256  string `json:"sha256"`
	Read    int    `json:"sites_read"`
	Fetched string `json:"fetched"`
}

var sources = []struct{ name, url, license string }{
	{"wmn", "https://raw.githubusercontent.com/WebBreacher/WhatsMyName/main/wmn-data.json", "CC BY-SA 4.0 (Copyright (C) 2015-2026 Micah Hoffman and contributors; project archived 2026-11-09)"},
	{"sherlock", "https://raw.githubusercontent.com/sherlock-project/sherlock/master/sherlock_project/resources/data.json", "MIT (Sherlock Project)"},
	{"maigret", "https://raw.githubusercontent.com/soxoj/maigret/main/maigret/resources/data.json", "MIT (soxoj/maigret)"},
}

func main() {
	fetch := flag.Bool("fetch", false, "download the source lists")
	in := flag.String("in", "", "directory holding wmn.json, sherlock.json, maigret.json")
	out := flag.String("out", "internal/modules/social/sites_default.json.gz", "output (gzipped JSON)")
	lock := flag.String("lock", "data/sites/sources.lock.json", "where to record the sources")
	flag.Parse()
	if !*fetch && *in == "" {
		fmt.Fprintln(os.Stderr, "need -fetch or -in DIR")
		os.Exit(2)
	}

	raw := map[string][]byte{}
	var recs []source
	for _, s := range sources {
		var b []byte
		var err error
		if *in != "" {
			b, err = os.ReadFile(filepath.Join(*in, s.name+".json"))
		} else {
			b, err = download(s.url)
		}
		if err != nil {
			die("%s: %v", s.name, err)
		}
		raw[s.name] = b
		h := sha256.Sum256(b)
		recs = append(recs, source{Name: s.name, URL: s.url, License: s.license, SHA256: hex.EncodeToString(h[:]), Fetched: time.Now().UTC().Format("2006-01-02")})
	}
	wmn, err := sitesync.ConvertWMN(raw["wmn"])
	check(err)
	sh, err := sitesync.ConvertSherlock(raw["sherlock"])
	check(err)
	mg, err := sitesync.ConvertMaigret(raw["maigret"])
	check(err)
	recs[0].Read, recs[1].Read, recs[2].Read = len(wmn), len(sh), len(mg)

	merged := sitesync.Merge(wmn, sh, mg)
	byTier, bySource := sitesync.Stats(merged)
	fmt.Printf("converted: wmn=%d sherlock=%d maigret=%d -> %d unique sites\n", len(wmn), len(sh), len(mg), len(merged))
	fmt.Printf("tiers: core=%d extended=%d all-only=%d | winners by source: %v\n", byTier[1], byTier[2], byTier[3], bySource)

	doc := struct {
		Generated string        `json:"generated"`
		Sources   []source      `json:"sources"`
		Sites     []social.Site `json:"sites"`
	}{time.Now().UTC().Format(time.RFC3339), recs, merged}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	enc := json.NewEncoder(zw)
	check(enc.Encode(doc))
	check(zw.Close())
	check(os.WriteFile(*out, buf.Bytes(), 0o644))
	fmt.Printf("wrote %s (%d bytes)\n", *out, buf.Len())

	lb, _ := json.MarshalIndent(map[string]any{"generated": doc.Generated, "total_sites": len(merged), "sources": recs}, "", "  ")
	check(os.MkdirAll(filepath.Dir(*lock), 0o755))
	check(os.WriteFile(*lock, append(lb, '\n'), 0o644))
}

func download(u string) ([]byte, error) {
	c := &http.Client{Timeout: 90 * time.Second}
	resp, err := c.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func check(err error) {
	if err != nil {
		die("%v", err)
	}
}

func die(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "sitesync: "+f+"\n", a...)
	os.Exit(1)
}
