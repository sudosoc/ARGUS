package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsAPIMasksKeysAndTests(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer llm.Close()
	ts, _ := newTestServerWith(t)

	put := func(path, body string) string {
		resp := do(t, "PUT", ts.URL+path, body, authed())
		defer resp.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		if resp.StatusCode != 200 {
			t.Fatalf("PUT %s: %d %s", path, resp.StatusCode, sb.String())
		}
		return sb.String()
	}
	out := put("/api/settings/google", `{"enabled":true,"key":"AIza-very-secret-key-9876"}`)
	if strings.Contains(out, "very-secret") || !strings.Contains(out, "••••9876") || !strings.Contains(out, `"key_source":"stored"`) {
		t.Fatalf("key leaked or not masked: %s", out)
	}
	out = put("/api/settings/llm", `{"provider":"local","model":"llama3"}`)
	if !strings.Contains(out, `"provider":"local"`) {
		t.Fatalf("llm choice: %s", out)
	}

	resp := do(t, "POST", ts.URL+"/api/settings/local/test", `{"base_url":"`+llm.URL+`","model":"llama3"}`, authed())
	defer resp.Body.Close()
	var b strings.Builder
	buf := make([]byte, 2048)
	n, _ := resp.Body.Read(buf)
	b.Write(buf[:n])
	if !strings.Contains(b.String(), `"ok":true`) {
		t.Fatalf("test connection: %s", b.String())
	}

	// unauthenticated access is refused like every other API call
	r2 := do(t, "GET", ts.URL+"/api/settings", "", nil)
	defer r2.Body.Close()
	if r2.StatusCode == 200 {
		t.Fatal("settings readable without the access token")
	}
	// unknown provider
	r3 := do(t, "PUT", ts.URL+"/api/settings/nope", `{"enabled":true}`, authed())
	defer r3.Body.Close()
	if r3.StatusCode != 400 {
		t.Fatalf("unknown provider: %d", r3.StatusCode)
	}
}

func TestUploadAnalysesFile(t *testing.T) {
	ts, _ := newTestServerWith(t)
	h := authed()
	h["Content-Type"] = "application/octet-stream"
	h["X-Filename"] = "cv%20layla.txt"
	resp := do(t, "POST", ts.URL+"/api/uploads", "Layla Hassan\nlayla@hassanlabs.org +20 100 555 0199\n", h)
	defer resp.Body.Close()
	buf := make([]byte, 8192)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if resp.StatusCode != 200 || !strings.Contains(body, `"kind":"text"`) || !strings.Contains(body, "layla@hassanlabs.org") || !strings.Contains(body, "cv layla.txt") {
		t.Fatalf("%d %s", resp.StatusCode, body)
	}
	h["X-Filename"] = "x.bin"
	r2 := do(t, "POST", ts.URL+"/api/uploads", "\x00\x01\x02\x03\x00\x00\x09\x09", h)
	defer r2.Body.Close()
	if r2.StatusCode != 400 {
		t.Fatalf("binary blob: %d", r2.StatusCode)
	}
	// without the CSRF header the upload is refused like every other write
	r3 := do(t, "POST", ts.URL+"/api/uploads", "x", map[string]string{"Cookie": cookieName + "=" + testToken})
	defer r3.Body.Close()
	if r3.StatusCode != 403 {
		t.Fatalf("csrf: %d", r3.StatusCode)
	}
}
