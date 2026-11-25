// Package util holds the small helpers that every collector needs: tolerant
// JSON fetching, string clipping and cleaning, and sorted map keys. Keeping
// them here (instead of a private copy in each module) means a fix lands once.
package util

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
)

// GetOption tunes GetJSON.
type GetOption func(*httpx.Request)

// Timeout overrides the per-request timeout (default 20s).
func Timeout(d time.Duration) GetOption { return func(r *httpx.Request) { r.Timeout = d } }

// MaxBody overrides the response size cap (default 4 MiB).
func MaxBody(n int64) GetOption { return func(r *httpx.Request) { r.MaxBody = n } }

// Header adds a request header.
func Header(k, v string) GetOption {
	return func(r *httpx.Request) {
		if r.Headers == nil {
			r.Headers = map[string]string{}
		}
		r.Headers[k] = v
	}
}

// GetJSON fetches u and decodes a 200 JSON response into out. It reports
// false on any transport error, non-200 status or malformed JSON; callers that
// need the status code should use httpx directly.
func GetJSON(ctx context.Context, c *httpx.Client, u string, out any, opts ...GetOption) bool {
	_, ok := GetJSONStatus(ctx, c, u, out, opts...)
	return ok
}

// GetJSONStatus is GetJSON that also returns the HTTP status (0 on transport error).
func GetJSONStatus(ctx context.Context, c *httpx.Client, u string, out any, opts ...GetOption) (int, bool) {
	req := httpx.Request{URL: u, Timeout: 20 * time.Second, MaxBody: 4 << 20, Headers: map[string]string{"Accept": "application/json"}}
	for _, o := range opts {
		o(&req)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return 0, false
	}
	if resp.StatusCode != 200 {
		return resp.StatusCode, false
	}
	return 200, json.Unmarshal(resp.Body, out) == nil
}

// MinClip is the least Clip and ClipList ever cut to: callers pass display-sized
// limits (120, 300…), but nothing a source returned is dropped below this size.
const MinClip = 5000

// Clip trims s and shortens it to at most n runes (never below MinClip), appending an ellipsis.
func Clip(s string, n int) string {
	if n < MinClip {
		n = MinClip
	}
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ClipList returns at most n items of s (never fewer than MinClip).
func ClipList[T any](s []T, n int) []T {
	if n < MinClip {
		n = MinClip
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

// NonEmpty returns the trimmed, non-blank values in order.
func NonEmpty(v ...string) []string {
	var out []string
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// FirstNonEmpty returns the first value that is not blank, trimmed.
func FirstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// Put stores v under k when it is not blank.
func Put(d map[string]string, k, v string) {
	if v = strings.TrimSpace(v); v != "" {
		d[k] = v
	}
}

// SortedKeys returns the keys of a string-keyed set or map in order.
func SortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Contains reports whether list holds v, ignoring case.
func Contains(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}
