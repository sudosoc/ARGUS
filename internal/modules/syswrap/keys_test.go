// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package syswrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

// fakeEye is a program that prints the GOOGLE_API_KEY it was given.
func fakeEye(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "the3rdeye")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho \"key=$GOOGLE_API_KEY https://example.org/x\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func runEye(t *testing.T, keys func(string) (string, bool)) []models.Finding {
	s, err := New(Config{Tools: []string{"the-3rd-eye"}, Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	var got []models.Finding
	p := pool.New(2, 16)
	defer p.Close()
	err = s.Scan(context.Background(), models.Target{Username: "layla"}, scanner.Deps{Pool: p}, func(f models.Finding) { got = append(got, f) })
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAPIToolRunsWithKeyInEnvironment(t *testing.T) {
	fakeEye(t)
	got := runEye(t, func(id string) (string, bool) { return "G-KEY-123", id == "google" })
	var run string
	for _, f := range got {
		if f.Category == "toolrun" {
			run = f.Details["output"]
		}
	}
	if !strings.Contains(run, "key=G-KEY-123") {
		t.Fatalf("key not passed in the environment: %q", run)
	}
}

func TestAPIToolSkippedWhenDisabledOrMissing(t *testing.T) {
	fakeEye(t)
	for name, keys := range map[string]func(string) (string, bool){
		"disabled": func(string) (string, bool) { return "K", false },
		"nokey":    func(string) (string, bool) { return "", true },
		"nil":      nil,
	} {
		got := runEye(t, keys)
		if len(got) != 1 || got[0].Details["skipped"] == "" {
			t.Fatalf("%s: expected one skipped note, got %+v", name, got)
		}
	}
}
