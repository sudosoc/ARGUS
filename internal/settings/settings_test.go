// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package settings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestStoreEncryptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update("openai", ptr(true), ptr("sk-secret-1234567890"), ptr("gpt-4o"), nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "settings.enc"))
	if strings.Contains(string(raw), "sk-secret") {
		t.Fatal("key is stored in clear text")
	}
	if fi, _ := os.Stat(filepath.Join(dir, "settings.enc")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v", fi.Mode().Perm())
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	k, en := s2.Resolve("openai")
	if k != "sk-secret-1234567890" || !en {
		t.Fatalf("resolve: %q %v", k, en)
	}
	for _, v := range s2.Views() {
		if v.ID == "openai" && (v.KeyHint != "••••7890" || v.KeySource != "stored") {
			t.Fatalf("view: %+v", v)
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARGUS_SETTINGS_KEY", "one")
	s, _ := Open(dir)
	_ = s.Update("openai", ptr(true), ptr("k"), nil, nil)
	t.Setenv("ARGUS_SETTINGS_KEY", "two")
	if _, err := Open(dir); err == nil {
		t.Fatal("expected a decryption error")
	}
}

func TestEnvFallbackAndClear(t *testing.T) {
	s, _ := Open("")
	s.env = func(k string) string {
		if k == "GOOGLE_API_KEY" {
			return "env-key-abcdef"
		}
		return ""
	}
	if k, _ := s.Resolve("google"); k != "env-key-abcdef" {
		t.Fatalf("env fallback: %q", k)
	}
	_ = s.Update("google", ptr(true), ptr("stored-key-xyz"), nil, nil)
	if k, _ := s.Resolve("google"); k != "stored-key-xyz" {
		t.Fatalf("stored should win: %q", k)
	}
	_ = s.Update("google", nil, ptr(""), nil, nil)
	if k, _ := s.Resolve("google"); k != "env-key-abcdef" {
		t.Fatalf("after clear: %q", k)
	}
	if err := s.Update("google", nil, ptr("a\nb"), nil, nil); err == nil {
		t.Fatal("newline in key accepted")
	}
}

func TestLLMConnectionProviders(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization") + r.Header.Get("x-api-key") + r.Header.Get("x-goog-api-key")
		gotPath = r.URL.Path
		if strings.Contains(gotAuth, "bad") {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key bad-key provided"}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	ctx := context.Background()
	if err := testLLM(ctx, "openai", "good", "m", srv.URL); err != nil || gotPath != "/v1/chat/completions" || gotAuth != "Bearer good" {
		t.Fatalf("openai: %v %s %s", err, gotPath, gotAuth)
	}
	if err := testLLM(ctx, "anthropic", "good", "m", srv.URL); err != nil || gotPath != "/v1/messages" || gotAuth != "good" {
		t.Fatalf("anthropic: %v %s %s", err, gotPath, gotAuth)
	}
	if err := testLLM(ctx, "gemini", "good", "gemini-x", srv.URL); err != nil || !strings.HasSuffix(gotPath, ":generateContent") {
		t.Fatalf("gemini: %v %s", err, gotPath)
	}
	if err := testLLM(ctx, "local", "", "llama3", srv.URL); err != nil || gotPath != "/v1/models" {
		t.Fatalf("local: %v %s", err, gotPath)
	}
	err := testLLM(ctx, "openai", "bad-key", "m", srv.URL)
	if err == nil || strings.Contains(err.Error(), "bad-key") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("error should mention 401 and hide the key: %v", err)
	}
	if err := testLLM(ctx, "openai", "", "m", srv.URL); err == nil {
		t.Fatal("empty key accepted")
	}
	if err := testLLM(ctx, "google", "k", "m", srv.URL); err == nil {
		t.Fatal("non-LLM provider accepted")
	}
}
