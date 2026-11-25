package syswrap

import (
	"os"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
)

// Tool describes how to run one external CLI program and turn its stdout into
// Findings. Each element returned by Args is one invocation's argument list.
type Tool struct {
	Name     string
	Binary   string
	Requires []string // Inputs fields that must be non-empty: username, email, domain, phone
	Timeout  time.Duration
	Args     func(Inputs) [][]string
	Parse    func(in Inputs, args []string, stdout string) []models.Finding

	// ArgsDir/ParseDir serve tools that write their result to files instead of
	// stdout (Maigret's JSON report). The runner gives each invocation a private
	// temporary directory, substituted for dirPlaceholder in the arguments,
	// hands it to ParseDir after the run and removes it afterwards.
	// APIProvider names a settings provider whose key this tool needs (for example "google").
	// The key is passed in the environment variable of that provider; when the provider is
	// disabled or has no key the tool is skipped and the run says so.
	APIProvider string

	ArgsDir  func(in Inputs, dir string) [][]string
	ParseDir func(in Inputs, dir, stdout string) []models.Finding
}

// dirPlaceholder stands for a job's private output directory in ArgsDir results.
const dirPlaceholder = "{{ARGUS_DIR}}"

func (t *Tool) applicable(in Inputs) bool {
	for _, r := range t.Requires {
		if in.get(r) == "" {
			return false
		}
	}
	return true
}

// builtinTools is the registry of wrappers that ship with argus.
func builtinTools() []*Tool {
	return []*Tool{
		{
			Name: "whois", Binary: "whois", Requires: []string{"domain"}, Timeout: 30 * time.Second,
			Args:  func(in Inputs) [][]string { return [][]string{{in.Domain}} },
			Parse: parseWhois,
		},
		{
			Name: "dig", Binary: "dig", Requires: []string{"domain"}, Timeout: 15 * time.Second,
			Args: func(in Inputs) [][]string {
				var a [][]string
				for _, typ := range []string{"MX", "TXT", "NS"} {
					a = append(a, []string{"+short", "+time=5", "+tries=1", typ, in.Domain})
				}
				return a
			},
			Parse: parseDig,
		},
		{
			Name: "theHarvester", Binary: "theHarvester", Requires: []string{"domain"}, Timeout: 180 * time.Second,
			Args: func(in Inputs) [][]string {
				return [][]string{{"-d", in.Domain, "-b", "crtsh,rapiddns,hackertarget", "-l", "200"}}
			},
			Parse: parseHarvester,
		},
		{
			Name: "subfinder", Binary: "subfinder", Requires: []string{"domain"}, Timeout: 180 * time.Second,
			Args:  func(in Inputs) [][]string { return [][]string{{"-d", in.Domain, "-silent"}} },
			Parse: parseSubfinder,
		},
		{
			Name: "whatweb", Binary: "whatweb", Requires: []string{"domain"}, Timeout: 90 * time.Second,
			Args: func(in Inputs) [][]string {
				return [][]string{{"-q", "--color=never", "--no-errors", "-a", "1", in.Domain}}
			},
			Parse: parseWhatweb,
		},
		{
			// Maigret's own JSON report carries the profile data it extracts
			// (names, bios, pictures, linked accounts), not just "found" lines.
			Name: "maigret", Binary: "maigret", Requires: []string{"username"}, Timeout: 30 * time.Minute,
			ArgsDir: func(in Inputs, dir string) [][]string {
				return [][]string{{"--no-color", "--no-progressbar", "--timeout", "10", "--retries", "1", "-n", "60",
					"-a", "--json", "ndjson", "--folderoutput", dir, in.Username}}
			},
			ParseDir: parseMaigretDir,
		},
		{
			Name: "holehe", Binary: "holehe", Requires: []string{"email"}, Timeout: 600 * time.Second,
			Args:  func(in Inputs) [][]string { return [][]string{{"--no-color", in.Email}} },
			Parse: parseHolehe,
		},
		{
			Name: "phoneinfoga", Binary: "phoneinfoga", Requires: []string{"phone"}, Timeout: 120 * time.Second,
			Args:  func(in Inputs) [][]string { return [][]string{{"scan", "-n", in.Phone}} },
			Parse: parseRawSummary("phoneinfoga", "phone"),
		},
		{
			Name: "sherlock", Binary: "sherlock", Requires: []string{"username"}, Timeout: 240 * time.Second,
			Args: func(in Inputs) [][]string {
				return [][]string{{"--print-found", "--no-color", "--timeout", "10", in.Username}}
			},
			Parse: parseSherlock,
		},
	}
}

// binary is the program to run: the built-in name unless ARGUS_TOOL_<NAME>_BIN overrides it.
func (t *Tool) binary() string {
	if v := os.Getenv("ARGUS_TOOL_" + envName(t.Name) + "_BIN"); v != "" {
		return v
	}
	return t.Binary
}

// envName turns a tool name into the form used in environment variables ("instagram-osint" -> "INSTAGRAM_OSINT").
func envName(n string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(n))
}
