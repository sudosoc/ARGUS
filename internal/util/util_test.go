// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/sudosoc/argus/internal/httpx"
)

func TestClip(t *testing.T) {
	if Clip("  abc  ", 5) != "abc" || Clip("héllo wörld", 5) != "héllo wörld" || Clip("", 3) != "" {
		t.Error("Clip must not cut anything shorter than MinClip, whatever limit the caller passes")
	}
	long := strings.Repeat("é", MinClip+10)
	if got := Clip(long, 5); len([]rune(got)) != MinClip+1 || !strings.HasSuffix(got, "…") {
		t.Errorf("a huge value is cut rune-safely at MinClip and marked: %d", len([]rune(got)))
	}
}

func TestListHelpers(t *testing.T) {
	if !reflect.DeepEqual(NonEmpty(" a ", "", "  ", "b"), []string{"a", "b"}) {
		t.Error("NonEmpty")
	}
	if FirstNonEmpty("", " x ", "y") != "x" || FirstNonEmpty() != "" {
		t.Error("FirstNonEmpty")
	}
	if !reflect.DeepEqual(ClipList([]string{"a", "b", "c"}, 2), []string{"a", "b", "c"}) || len(ClipList[string](nil, 2)) != 0 {
		t.Error("ClipList must not drop items below MinClip")
	}
	if got := ClipList(make([]int, MinClip+3), 1); len(got) != MinClip {
		t.Errorf("ClipList cap: %d", len(got))
	}
	d := map[string]string{}
	Put(d, "a", "  ")
	Put(d, "b", " v ")
	if len(d) != 1 || d["b"] != "v" {
		t.Errorf("Put: %v", d)
	}
	if !reflect.DeepEqual(SortedKeys(map[string]bool{"b": true, "a": false}), []string{"a", "b"}) {
		t.Error("SortedKeys")
	}
	if !Contains([]string{"Go"}, "go") || Contains(nil, "x") {
		t.Error("Contains")
	}
}

func TestGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			if r.Header.Get("X-A") != "1" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write([]byte(`{"n":3}`))
		case "/bad":
			_, _ = w.Write([]byte(`not json`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c, _ := httpx.New(httpx.Options{Retries: 0})
	var v struct{ N int }
	if !GetJSON(context.Background(), c, srv.URL+"/ok", &v, Header("X-A", "1")) || v.N != 3 {
		t.Errorf("ok: %+v", v)
	}
	if GetJSON(context.Background(), c, srv.URL+"/bad", &v) {
		t.Error("malformed JSON must report false")
	}
	if code, ok := GetJSONStatus(context.Background(), c, srv.URL+"/missing", &v); ok || code != 404 {
		t.Errorf("status passthrough: %d %v", code, ok)
	}
}
