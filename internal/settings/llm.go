// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TestLLMConnection sends a one-token request to the provider's real endpoint and
// reports whether the key and model work. The key is only ever sent to that provider.
func TestLLMConnection(ctx context.Context, provider, key, model string) error {
	return testLLM(ctx, provider, key, model, "")
}

// testLLM is TestLLMConnection with an overridable base URL (the local server's
// address, and the endpoint of the test server in unit tests).
func testLLM(ctx context.Context, provider, key, model, base string) error {
	p, ok := ProviderByID(provider)
	if !ok || p.Kind != "llm" {
		return errors.New("not a model provider")
	}
	if p.NeedsKey && strings.TrimSpace(key) == "" {
		return errors.New("no API key")
	}
	if model == "" {
		model = p.DefaultModel
	}
	if model == "" {
		return errors.New("no model name")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var (
		req *http.Request
		err error
	)
	jsonBody := func(v any) io.Reader { b, _ := json.Marshal(v); return bytes.NewReader(b) }
	switch provider {
	case "openai":
		req, err = http.NewRequestWithContext(ctx, "POST", orDefault(base, "https://api.openai.com")+"/v1/chat/completions",
			jsonBody(map[string]any{"model": model, "max_tokens": 1, "messages": []map[string]string{{"role": "user", "content": "ping"}}}))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	case "anthropic":
		req, err = http.NewRequestWithContext(ctx, "POST", orDefault(base, "https://api.anthropic.com")+"/v1/messages",
			jsonBody(map[string]any{"model": model, "max_tokens": 1, "messages": []map[string]string{{"role": "user", "content": "ping"}}}))
		if err == nil {
			req.Header.Set("x-api-key", key)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case "gemini":
		u := orDefault(base, "https://generativelanguage.googleapis.com") + "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
		req, err = http.NewRequestWithContext(ctx, "POST", u,
			jsonBody(map[string]any{"contents": []map[string]any{{"parts": []map[string]string{{"text": "ping"}}}}, "generationConfig": map[string]int{"maxOutputTokens": 1}}))
		if err == nil {
			req.Header.Set("x-goog-api-key", key)
		}
	case "local":
		b := strings.TrimRight(orDefault(base, p.DefaultBase), "/")
		if u, perr := url.Parse(b); perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("the server address must be a full http:// or https:// URL")
		}
		req, err = http.NewRequestWithContext(ctx, "GET", b+"/v1/models", nil)
		if err == nil && strings.TrimSpace(key) != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the server: %s", scrub(err.Error(), key))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, scrub(briefError(body), key))
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimRight(strings.TrimSpace(v), "/")
	}
	return d
}

// briefError pulls the message out of a provider's JSON error body.
func briefError(b []byte) string {
	var j struct {
		Error any `json:"error"`
	}
	if json.Unmarshal(b, &j) == nil {
		switch e := j.Error.(type) {
		case string:
			return e
		case map[string]any:
			if m, ok := e["message"].(string); ok {
				return m
			}
		}
	}
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// scrub makes sure an error message never contains the key.
func scrub(msg, key string) string {
	if key != "" {
		msg = strings.ReplaceAll(msg, key, "•••")
	}
	return msg
}

// TestLLMConnectionAt is TestLLMConnection with a server address, used for a local model.
func TestLLMConnectionAt(ctx context.Context, provider, key, model, base string) error {
	if provider != "local" {
		base = "" // hosted providers are only ever contacted at their own endpoints
	}
	return testLLM(ctx, provider, key, model, base)
}
