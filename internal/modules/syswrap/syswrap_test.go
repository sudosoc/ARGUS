package syswrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

func TestInputsRejectInjection(t *testing.T) {
	in := inputsFrom(models.Target{Username: "--output=/etc/x", Email: "a@evil.com; rm -rf /", Phone: "+1555123456"})
	if in.Username != "" || in.Email != "" || in.Domain != "" {
		t.Fatalf("unsafe values accepted: %+v", in)
	}
	if in.Phone == "" {
		t.Fatal("valid phone rejected")
	}
	in = inputsFrom(models.Target{Email: "Bob@Example.COM"})
	if in.Domain != "example.com" {
		t.Fatalf("domain = %q", in.Domain)
	}
}

func TestParseWhois(t *testing.T) {
	out := "Registrar: Example Registrar, Inc.\nCreation Date: 2001-01-01T00:00:00Z\nName Server: NS1.EXAMPLE.COM\nName Server: ns2.example.com\nName Server: ns1.example.com\nRegistrant Email: REDACTED FOR PRIVACY\n"
	f := parseWhois(Inputs{Domain: "example.com"}, nil, out)
	if len(f) != 1 || f[0].Details["name_servers"] != "NS1.EXAMPLE.COM, ns2.example.com" || f[0].Details["registrar"] == "" {
		t.Fatalf("%+v", f)
	}
	if _, has := f[0].Details["registrant_email"]; has {
		t.Fatal("redacted value kept")
	}
}

func TestParseHarvester(t *testing.T) {
	out := "[*] Emails found: 1\n----------------------\nadmin@example.com\n\n[*] Hosts found: 2\n---------------------\nwww.example.com:93.184.216.34\nmail.example.com\n"
	f := parseHarvester(Inputs{Domain: "example.com"}, nil, out)
	if len(f) != 3 || f[0].Details["email"] != "admin@example.com" || f[1].URL != "www.example.com" || f[1].Details["ip"] != "93.184.216.34" {
		t.Fatalf("%+v", f)
	}
}

func TestParseSherlockAndDig(t *testing.T) {
	f := parseSherlock(Inputs{Username: "bob"}, nil, "[*] Checking\n[+] GitHub: https://github.com/bob\n")
	if len(f) != 1 || f[0].Source != "GitHub" || f[0].URL != "https://github.com/bob" {
		t.Fatalf("%+v", f)
	}
	d := parseDig(Inputs{Domain: "x.org"}, []string{"+short", "MX", "x.org"}, "10 mail.x.org.\n")
	if len(d) != 1 || d[0].Details["type"] != "MX" {
		t.Fatalf("%+v", d)
	}
}

func TestCustomTool(t *testing.T) {
	sp := CustomSpec{Name: "demo", Binary: "demo", Args: []string{"-u", "{username}"}, Pattern: `^\[\+\]\s+(?P<title>\S+)\s+(?P<url>https?://\S+)$`}
	tool, err := sp.build()
	if err != nil {
		t.Fatal(err)
	}
	if tool.applicable(Inputs{}) || !tool.applicable(Inputs{Username: "u"}) {
		t.Fatal("requirements not derived from placeholders")
	}
	if got := tool.Args(Inputs{Username: "bob"})[0]; got[1] != "bob" {
		t.Fatalf("args %v", got)
	}
	f := tool.Parse(Inputs{}, nil, "[+] Site https://s.example/bob\nnoise")
	if len(f) != 1 || f[0].Source != "Site" || f[0].URL != "https://s.example/bob" {
		t.Fatalf("%+v", f)
	}
}

func TestRunCmdAndTimeout(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not in PATH")
	}
	out, err := runCmd(context.Background(), goBin, []string{"version"}, 30*time.Second, 1<<16)
	if err != nil || !strings.HasPrefix(out, "go version") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := runCmd(context.Background(), goBin, []string{"version"}, time.Nanosecond, 1<<16); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestParseMaigretDir(t *testing.T) {
	dir := t.TempDir()
	lines := `{"sitename":"GitHub","url_user":"https://github.com/bob","rank":80,"status":{"site_name":"GitHub","url":"https://github.com/bob","status":"Claimed","tags":["coding"],"ids":{"fullname":"Bob Builder","image":"https://avatars.example/bob.png","bio":"builds things","location":"Cairo","email":"Bob@Example.com","links":"[]","uid":"42"}},"ids_usernames":{"bobby":"username","bob":"username"}}
{"sitename":"Plain","url_user":"https://plain.example/bob","status":{"site_name":"Plain","url":"https://plain.example/bob","status":"Claimed","ids":{}}}
not json
{"sitename":"","url_user":"","status":{}}
`
	if err := os.WriteFile(filepath.Join(dir, "report_bob_ndjson.json"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	res := parseMaigretDir(Inputs{Username: "bob"}, dir, "")
	if len(res) != 2 {
		t.Fatalf("want 2 findings, got %d: %+v", len(res), res)
	}
	g := res[0]
	if g.Source != "GitHub" || g.Details["name"] != "Bob Builder" || g.Details["bio"] != "builds things" || g.Confidence < 0.8 {
		t.Errorf("profile data lost: %+v", g)
	}
	var kinds []string
	for _, e := range g.Entities {
		kinds = append(kinds, e.Type+":"+e.Value)
	}
	for _, want := range []string{"name:Bob Builder", "email:bob@example.com", "location:Cairo", "username:bobby"} {
		found := false
		for _, k := range kinds {
			found = found || k == want
		}
		if !found {
			t.Errorf("entity %q missing in %v", want, kinds)
		}
	}
	for _, k := range kinds {
		if k == "username:bob" {
			t.Error("the searched username must not be re-reported as a new lead")
		}
	}
	if res[1].Confidence != 0.7 || res[1].Source != "Plain" {
		t.Errorf("plain account: %+v", res[1])
	}
}

func TestArgsDirPlaceholderRoundTrip(t *testing.T) {
	var mg *Tool
	for _, tl := range builtinTools() {
		if tl.Name == "maigret" {
			mg = tl
		}
	}
	if mg == nil || mg.ArgsDir == nil || mg.ParseDir == nil {
		t.Fatal("maigret must use the file-based report")
	}
	args := mg.ArgsDir(Inputs{Username: "bob"}, dirPlaceholder)[0]
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, dirPlaceholder) || !strings.Contains(joined, "--json ndjson") || args[len(args)-1] != "bob" {
		t.Errorf("args: %q", joined)
	}
}
