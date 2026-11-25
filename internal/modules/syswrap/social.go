package syswrap

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// Social-media collection tools. Each is a separate Python/Go program that must be
// installed; none is bundled. Their command lines differ between versions, so every
// tool here has a default invocation and two overrides:
//
//	ARGUS_TOOL_<NAME>_BIN   the program to run
//	ARGUS_TOOL_<NAME>_ARGS  its arguments, space separated, with {username} {email} {domain} {phone}
//
// Whatever a tool prints is stored in full (the "toolrun" record); the generic parser
// additionally lists every URL, e-mail address and phone number found in that output.
// Use these only on public accounts you are allowed to examine and within each
// platform's terms.
func socialTools() []*Tool {
	return []*Tool{
		socialTool("facebook-scraper", "facebook-scraper", []string{"--pages", "3", "{username}"}, "username", "", 10*time.Minute),
		socialTool("instagram-osint", "igosint", []string{"{username}"}, "username", "", 10*time.Minute),
		socialTool("insta-osint-engine", "insta-osint-engine", []string{"{username}"}, "username", "", 15*time.Minute),
		socialTool("open-nami", "nami", []string{"{username}"}, "username", "", 20*time.Minute),
		// The 3rd Eye needs a Google API key: it runs only when the "google" provider is enabled in Settings.
		socialTool("the-3rd-eye", "the3rdeye", []string{"{username}"}, "username", "google", 15*time.Minute),
	}
}

func socialTool(name, bin string, defArgs []string, requires, api string, timeout time.Duration) *Tool {
	t := &Tool{Name: name, Binary: bin, Requires: []string{requires}, Timeout: timeout, APIProvider: api}
	t.Args = func(in Inputs) [][]string {
		tmpl := defArgs
		if v := strings.Fields(os.Getenv("ARGUS_TOOL_" + envName(name) + "_ARGS")); len(v) > 0 {
			tmpl = v
		}
		out := make([]string, len(tmpl))
		for i, a := range tmpl {
			a = strings.ReplaceAll(a, "{username}", in.Username)
			a = strings.ReplaceAll(a, "{email}", in.Email)
			a = strings.ReplaceAll(a, "{domain}", in.Domain)
			a = strings.ReplaceAll(a, "{phone}", in.Phone)
			out[i] = a
		}
		return [][]string{out}
	}
	t.Parse = func(in Inputs, args []string, out string) []models.Finding { return parseGeneric(name, in, out) }
	return t
}

var (
	urlRe    = regexp.MustCompile(`https?://[^\s"'<>)\]]+`)
	emailRe  = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	phoneRe2 = regexp.MustCompile(`\+\d[\d\s().-]{7,16}\d`)
)

// parseGeneric lists the URLs, e-mail addresses and phone numbers in a tool's
// output. It makes no claim about the format: it reads text.
func parseGeneric(tool string, in Inputs, out string) []models.Finding {
	uniq := func(a []string) []string {
		seen := map[string]bool{}
		var r []string
		for _, s := range a {
			s = strings.TrimRight(s, ".,;:")
			if !seen[s] {
				seen[s] = true
				r = append(r, s)
			}
		}
		sort.Strings(r)
		return r
	}
	urls, emails, phones := uniq(urlRe.FindAllString(out, -1)), uniq(emailRe.FindAllString(out, -1)), uniq(phoneRe2.FindAllString(out, -1))
	if len(urls)+len(emails)+len(phones) == 0 {
		return nil
	}
	d := map[string]string{"username": in.Username, "urls": strconv.Itoa(len(urls)), "e-mails": strconv.Itoa(len(emails)), "phones": strconv.Itoa(len(phones))}
	if len(urls) > 0 {
		d["URLs"] = strings.Join(urls, "\n")
	}
	if len(emails) > 0 {
		d["e-mail addresses"] = strings.Join(emails, "\n")
	}
	if len(phones) > 0 {
		d["phone numbers"] = strings.Join(phones, "\n")
	}
	f := finding(tool+": "+in.Username, "social-scraper", "", 0.5, d)
	for _, e := range emails {
		f.Entities = append(f.Entities, models.Entity{Type: "email", Value: strings.ToLower(e)})
	}
	return []models.Finding{f}
}
