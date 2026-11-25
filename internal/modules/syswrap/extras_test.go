package syswrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSpiderFoot(t *testing.T) {
	out := "banner line\n[{\"type\":\"EMAILADDR\",\"data\":\"a@x.example\",\"module\":\"sfp_hunter\",\"generated\":1},{\"type\":\"EMAILADDR\",\"data\":\"a@x.example\",\"module\":\"sfp_other\",\"generated\":2},{\"type\":\"INTERNET_NAME\",\"data\":\"www.x.example\",\"module\":\"sfp_dns\",\"generated\":3},{\"type\":\"ROOT\",\"data\":\"x.example\"}]"
	got := parseSpiderFoot(Inputs{}, []string{"-s", "x.example"}, out)
	if len(got) != 2 {
		t.Fatalf("findings: %+v", got)
	}
	if got[0].Details["count"] != "1" || !strings.Contains(got[0].Details["spiderfoot modules"], "hunter") {
		t.Fatalf("bad details: %+v", got[0].Details)
	}
}

func TestParseSpiderFootBadJSON(t *testing.T) {
	if got := parseSpiderFoot(Inputs{}, nil, "not json"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestParseH8mailHidesCredentials(t *testing.T) {
	dir := t.TempDir()
	js := `{"targets":[{"target":"a@x.example","pwn_num":2,"data":[["HIBP","Adobe"],["LEAKLOOKUP_PUB","a@x.example:hunter2"]]}]}`
	if err := os.WriteFile(filepath.Join(dir, "h8mail.json"), []byte(js), 0o600); err != nil {
		t.Fatal(err)
	}
	got := parseH8mail(Inputs{}, dir, "")
	if len(got) != 1 {
		t.Fatalf("findings: %+v", got)
	}
	all := got[0].Source
	for k, v := range got[0].Details {
		all += k + v
	}
	if strings.Contains(all, "hunter2") {
		t.Fatalf("credential leaked into the finding: %s", all)
	}
	if got[0].Details["records found"] != "2" || !strings.Contains(got[0].Details["sources"], "HIBP") {
		t.Fatalf("bad details: %+v", got[0].Details)
	}
}
