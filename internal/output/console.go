// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package output renders results: live coloured console output and
// JSON / CSV / TXT report export.
package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"

	"github.com/sudosoc/argus/internal/models"
)

var (
	cGreen  = color.New(color.FgGreen, color.Bold)
	cRed    = color.New(color.FgRed)
	cYellow = color.New(color.FgYellow)
	cCyan   = color.New(color.Bold) // brand and info lines: plain bold, no colour noise
	cDim    = color.New(color.Faint)
	cBold   = color.New(color.Bold)
)

// Console prints findings and progress. Safe for concurrent use.
type Console struct {
	mu       sync.Mutex
	out, err io.Writer
	progress atomic.Bool // a progress line is currently on screen
	live     map[string]*liveMod
}

// liveMod is what the console knows about one running collector.
type liveMod struct {
	done, total int
	now         string
}

// NewConsole writes results to stdout and progress/diagnostics to stderr.
func NewConsole(noColor bool) *Console {
	if noColor {
		color.NoColor = true
	}
	return &Console{out: os.Stdout, err: os.Stderr, live: map[string]*liveMod{}}
}

func (c *Console) clearProgress() {
	if c.progress.Swap(false) {
		fmt.Fprint(c.err, "\r\033[K")
	}
}

// Banner prints the program header.
func (c *Console) Banner(version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cCyan.Fprintf(c.err, "\n  SUDOSOC ARGUS v%s", version)
	cDim.Fprintln(c.err, "  — API-less OSINT reconnaissance")
	cDim.Fprintln(c.err, "  Use only on subjects you are lawfully permitted to investigate.")
	fmt.Fprintln(c.err)
}

// Info prints a status line to stderr.
func (c *Console) Info(format string, a ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearProgress()
	cCyan.Fprint(c.err, "[*] ")
	fmt.Fprintf(c.err, format+"\n", a...)
}

// Warn prints a warning line to stderr.
func (c *Console) Warn(format string, a ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearProgress()
	cYellow.Fprint(c.err, "[!] ")
	fmt.Fprintf(c.err, format+"\n", a...)
}

// Error prints an error line to stderr.
func (c *Console) Error(format string, a ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearProgress()
	cRed.Fprint(c.err, "[x] ")
	fmt.Fprintf(c.err, format+"\n", a...)
}

// Finding prints one positive result immediately: the headline plus a few
// of its most informative details.
func (c *Console) Finding(f models.Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearProgress()
	if f.Status == models.StatusFound {
		cGreen.Fprint(c.out, "[+] ")
	} else {
		cYellow.Fprint(c.out, "[?] ")
	}
	cDim.Fprintf(c.out, "%-9s ", f.Module)
	cBold.Fprint(c.out, f.Source)
	if f.URL != "" {
		fmt.Fprintf(c.out, "  %s", f.URL)
	}
	cDim.Fprintf(c.out, "  (%.0f%%)", f.Confidence*100)
	if f.Via != "" {
		cDim.Fprintf(c.out, "  via %s", f.Via)
	}
	fmt.Fprintln(c.out)
	for _, l := range detailLines(f.Details, 4) {
		cDim.Fprintf(c.out, "              %s\n", l)
	}
}

// Module reports a collector starting, finishing or failing, and what it last looked at.
// state is "running", "done" or "failed".
func (c *Console) Module(name, state string, found, errs int, now, errMsg string, took time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.live[name]
	switch state {
	case "running":
		if m == nil {
			c.clearProgress()
			cDim.Fprintf(c.err, "[>] %s started\n", name)
			c.live[name] = &liveMod{now: now}
			return
		}
		m.now = now
	case "done":
		c.clearProgress()
		cDim.Fprintf(c.err, "[✓] %s finished · %d found · %d errors · %.1fs\n", name, found, errs, took.Seconds())
		delete(c.live, name)
	case "failed":
		c.clearProgress()
		cRed.Fprintf(c.err, "[x] %s stopped: %s\n", name, errMsg)
		delete(c.live, name)
	}
}

// Progress draws one self-overwriting line that lists every collector that is running, with how far
// each has got (throttled).
func (c *Console) Progress(module string, done, total int) {
	if total <= 0 || (done%20 != 0 && done != total) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.live[module]
	if m == nil {
		m = &liveMod{}
		c.live[module] = m
	}
	m.done, m.total = done, total
	var names []string
	for n := range c.live {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "    running %d: ", len(names))
	for i, n := range names {
		l := c.live[n]
		part := n
		if l.total > 0 {
			part = fmt.Sprintf("%s %d%%", n, l.done*100/l.total)
		}
		if i > 0 {
			b.WriteString(" · ")
		}
		if b.Len()+len(part) > 118 {
			fmt.Fprintf(&b, "+%d more", len(names)-i)
			break
		}
		b.WriteString(part)
	}
	c.progress.Store(true)
	cDim.Fprintf(c.err, "\r\033[K%s", b.String())
}

// Summary prints the end-of-run table.
func (c *Console) Summary(r *models.Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearProgress()
	fmt.Fprintln(c.out)
	cCyan.Fprintln(c.out, "── Summary ──────────────────────────────")
	for _, m := range r.Modules {
		name := m.Name
		if m.Via != "" {
			name += " <- " + m.Via
		}
		fmt.Fprintf(c.out, "  %-10s checked=%-5d found=", name, m.Checked)
		cGreen.Fprintf(c.out, "%-4d", m.Found)
		fmt.Fprintf(c.out, " errors=%-4d %5.1fs", m.Errors, float64(m.DurationMS)/1000)
		if m.Error != "" {
			cRed.Fprintf(c.out, "  ERROR: %s", m.Error)
		}
		fmt.Fprintln(c.out)
	}
	for _, s := range r.Skipped {
		cDim.Fprintf(c.out, "  skipped    %s\n", s)
	}
	if r.Interrupted {
		cYellow.Fprintln(c.out, "  run interrupted — results are partial")
	}
	if in := r.Insights; in != nil && in.Brief.Headline != "" {
		fmt.Fprintln(c.out)
		cCyan.Fprintln(c.out, "── Insights ─────────────────────────────")
		fmt.Fprintf(c.out, "  %s  [%s]\n", in.Brief.Headline, in.Brief.Level)
		for i, a := range in.Actions {
			if i == 3 {
				break
			}
			fmt.Fprintf(c.out, "  next %d: %s\n", i+1, a.Title)
		}
		cDim.Fprintln(c.out, "  full brief, persona, authenticity and more: the web UI (Insights) or the JSON/HTML report")
	}
}
