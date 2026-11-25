package doctor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCheckAndPrint(t *testing.T) {
	have := map[string]string{"maigret": "/usr/local/bin/maigret", "chromium": "/usr/bin/chromium", "git": "/usr/bin/git"}
	rs := Check(func(n string) (string, error) {
		if p, ok := have[n]; ok {
			return p, nil
		}
		return "", errors.New("nf")
	})
	if len(rs) != len(Tools) {
		t.Fatalf("%d results", len(rs))
	}
	found := map[string]string{}
	for _, r := range rs {
		found[r.Name] = r.Path
	}
	if found["maigret"] == "" || found["chrome / chromium"] != "/usr/bin/chromium" || found["holehe"] != "" {
		t.Errorf("%v", found)
	}
	m := Missing(rs)
	if len(m) != len(Tools)-3 {
		t.Errorf("missing %v", m)
	}
	var b bytes.Buffer
	Print(&b, rs)
	out := b.String()
	for _, want := range []string{"[+] maigret", "[-] holehe", "get it:", "optional tools missing"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestParseConf(t *testing.T) {
	m := ParseConf(strings.NewReader("# c\nSRC_DIR=\"/home/u/argus\"\nMODE=user\n\nBAD LINE\nPREFIX=\n"))
	if m["SRC_DIR"] != "/home/u/argus" || m["MODE"] != "user" || m["PREFIX"] != "" || len(m) != 3 {
		t.Errorf("%v", m)
	}
}

func TestToolsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tl := range Tools {
		if tl.Name == "" || len(tl.Binaries) == 0 || tl.Adds == "" || tl.Install == "" || seen[tl.Name] {
			t.Errorf("bad tool entry %+v", tl)
		}
		seen[tl.Name] = true
	}
}
