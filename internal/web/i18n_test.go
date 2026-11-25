package web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The Arabic dictionary is a JSON object inside a script file. Every "{}" in a key must be carried by its translation.
func TestArabicDictionaryIsConsistent(t *testing.T) {
	b, err := os.ReadFile("static/js/i18n-ar.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "UI.AR = ")
	j := strings.Index(s, ";\n  // Short labels")
	if i < 0 || j < 0 {
		t.Fatal("dictionary markers not found")
	}
	var d map[string]string
	if err := json.Unmarshal([]byte(s[i+len("UI.AR = "):j]), &d); err != nil {
		t.Fatalf("dictionary is not valid JSON: %v", err)
	}
	if len(d) < 500 {
		t.Fatalf("dictionary looks truncated: %d entries", len(d))
	}
	for k, v := range d {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			t.Errorf("empty entry %q", k)
		}
		if strings.Count(k, "{}") != strings.Count(v, "{}") {
			t.Errorf("placeholder mismatch: %q -> %q", k, v)
		}
	}
}
