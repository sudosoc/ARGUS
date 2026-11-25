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
