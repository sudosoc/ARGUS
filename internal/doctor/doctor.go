// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package doctor reports which optional external tools and system services are
// available, what each one adds to an investigation, and how to get the missing
// ones. It also locates the source checkout an installation was built from, so
// `argus -update` can run the updater.
package doctor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tool is an optional program ARGUS can use.
type Tool struct {
	Name     string   // display name
	Binaries []string // executable names to look for, first match wins
	Adds     string   // what it adds to an investigation
	Install  string   // how to get it when missing
}

// Tools lists every optional program, in the order they are reported.
var Tools = []Tool{
	{"maigret", []string{"maigret"}, "username dossier: names, bios, pictures and linked accounts from thousands of sites", "./install.sh (or: python3 -m venv v && v/bin/pip install maigret)"},
	{"sherlock", []string{"sherlock"}, "second opinion on username presence", "./install.sh (or: pipx install sherlock-project)"},
	{"holehe", []string{"holehe"}, "which sites have an account registered with an e-mail address", "./install.sh (or: pipx install holehe)"},
	{"phoneinfoga", []string{"phoneinfoga"}, "extra phone-number scanners", "./install.sh (release binary from github.com/sundowndev/phoneinfoga)"},
	{"theHarvester", []string{"theHarvester", "theharvester"}, "e-mails and hosts of a domain from public sources", "apt install theharvester (Debian/Kali); the pip build needs Python >= 3.12"},
	{"subfinder", []string{"subfinder"}, "passive subdomain discovery", "go install github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest"},
	{"whatweb", []string{"whatweb"}, "web technology fingerprint", "apt install whatweb"},
	{"whois", []string{"whois"}, "registration records for domains", "apt install whois"},
	{"dig", []string{"dig"}, "DNS records (MX, TXT, NS) cross-check", "apt install dnsutils (bind-utils on Fedora)"},
	{"exiftool", []string{"exiftool"}, "deep metadata of downloaded pictures: device, software, author, serial numbers, GPS", "apt install libimage-exiftool-perl"},
	{"tesseract", []string{"tesseract"}, "OCR: reads text printed in downloaded pictures (signs, badges, screenshots); English and Arabic by default when the Arabic data is installed; change with ARGUS_OCR_LANGS", "apt install tesseract-ocr (and tesseract-ocr-ara for Arabic)"},
	{"ffmpeg", []string{"ffmpeg"}, "uploaded videos: frames are taken and read with OCR; ffprobe reads their metadata", "apt install ffmpeg"},
	{"easyocr", []string{"easyocr"}, "OCR fallback when tesseract is missing (pip install easyocr; downloads models on first use)", "pip install easyocr"},
	{"spiderfoot", []string{"spiderfoot"}, "all-in-one OSINT framework; run passively and its events are imported", "apt install spiderfoot (Kali) or see github.com/smicallef/spiderfoot"},
	{"h8mail", []string{"h8mail"}, "breach lookup for an e-mail: only source names and counts are shown, never credentials", "pip install h8mail (./install.sh does it in a private venv)"},
	{"chrome / chromium", []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"}, "-browser (logged-out LinkedIn view) and PDF reports", "apt install chromium (or chromium-browser)"},
	{"sqlite3", []string{"sqlite3"}, "load the SQL export: sqlite3 case.db < case.sql", "apt install sqlite3"},
	{"git", []string{"git"}, "updates (argus -update)", "apt install git"},
}

// Result is the state of one tool.
type Result struct {
	Tool
	Path string // empty when missing
}

// Check looks every tool up with lookPath (exec.LookPath in production).
func Check(lookPath func(string) (string, error)) []Result {
	out := make([]Result, 0, len(Tools))
	for _, t := range Tools {
		r := Result{Tool: t}
		for _, b := range t.Binaries {
			if p, err := lookPath(b); err == nil {
				r.Path = p
				break
			}
		}
		out = append(out, r)
	}
	return out
}

// Missing returns the names of the tools that were not found.
func Missing(rs []Result) []string {
	var m []string
	for _, r := range rs {
		if r.Path == "" {
			m = append(m, r.Name)
		}
	}
	return m
}

// Print writes a readable report.
func Print(w io.Writer, rs []Result) {
	fmt.Fprintln(w, "Optional tools (ARGUS works without them; each one adds depth):")
	for _, r := range rs {
		if r.Path != "" {
			fmt.Fprintf(w, "  [+] %-18s %s\n", r.Name, r.Path)
		} else {
			fmt.Fprintf(w, "  [-] %-18s missing: %s\n", r.Name, r.Adds)
			fmt.Fprintf(w, "      %-18s get it: %s\n", "", r.Install)
		}
	}
	if m := Missing(rs); len(m) > 0 {
		fmt.Fprintf(w, "\n%d of %d optional tools missing. Run ./install.sh (or ./update.sh) to install what can be installed.\n", len(m), len(rs))
	} else {
		fmt.Fprintln(w, "\nAll optional tools are installed.")
	}
}

// LookPath is exec.LookPath that also searches the directories the installer
// uses even when they are not on PATH (so a fresh --user install is detected).
func LookPath(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, d := range []string{filepath.Join(home, ".local", "bin"), "/usr/local/bin", "/opt/pipx/bin"} {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", exec.ErrNotFound
}

// ConfPaths are the places install.sh records how and where ARGUS was installed.
func ConfPaths() []string {
	var out []string
	if d := os.Getenv("ARGUS_DATA"); d != "" {
		out = append(out, filepath.Join(d, "install.conf"))
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		out = append(out, filepath.Join(x, "argus", "install.conf"))
	}
	if h, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(h, ".local", "share", "argus", "install.conf"))
	}
	return append(out, "/usr/local/share/argus/install.conf", "/usr/share/argus/install.conf")
}

// ParseConf reads KEY=VALUE lines (install.conf); values may be quoted.
func ParseConf(r io.Reader) map[string]string {
	m := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return m
}

// FindSource returns the source directory recorded by the installer.
func FindSource() (string, error) {
	for _, p := range ConfPaths() {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		conf := ParseConf(f)
		f.Close()
		if src := conf["SRC_DIR"]; src != "" {
			if _, err := os.Stat(filepath.Join(src, "update.sh")); err == nil {
				return src, nil
			}
		}
	}
	return "", fmt.Errorf("cannot find the source checkout this copy was installed from (no install.conf); run ./update.sh from the source directory")
}
