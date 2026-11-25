// Package syswrap runs external Linux/CLI OSINT tools via os/exec (never
// through a shell) and converts their output into the engine's Findings.
package syswrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

// Config tunes the module.
type Config struct {
	Tools      []string      // restrict to these tool names; empty = all installed
	ToolsFile  string        // extra user-defined wrappers (JSON, see CustomSpec)
	MaxProcs   int           // concurrent child processes (default 4)
	MaxOutput  int           // bytes of stdout kept per run (default 64 MiB); anything beyond is flagged as truncated
	TimeoutCap time.Duration // upper bound for any tool's timeout; 0 = tool default
}

// Scanner is the system-wrapper module.
type Scanner struct {
	cfg   Config
	tools []*Tool
}

var _ scanner.Scanner = (*Scanner)(nil)

// New builds the module from the built-in registry plus any custom tools.
func New(cfg Config) (*Scanner, error) {
	if cfg.MaxProcs <= 0 {
		cfg.MaxProcs = 4
	}
	if cfg.MaxOutput <= 0 {
		cfg.MaxOutput = 64 << 20
	}
	tools := builtinTools()
	if cfg.ToolsFile != "" {
		custom, err := loadCustomTools(cfg.ToolsFile)
		if err != nil {
			return nil, err
		}
		tools = append(tools, custom...)
	}
	if len(cfg.Tools) > 0 {
		want := map[string]bool{}
		for _, n := range cfg.Tools {
			want[strings.ToLower(n)] = true
		}
		var sel []*Tool
		for _, t := range tools {
			if want[strings.ToLower(t.Name)] {
				sel = append(sel, t)
				delete(want, strings.ToLower(t.Name))
			}
		}
		for n := range want {
			return nil, fmt.Errorf("unknown tool %q", n)
		}
		tools = sel
	}
	return &Scanner{cfg: cfg, tools: tools}, nil
}

func (s *Scanner) Name() string { return "syswrap" }
func (s *Scanner) Description() string {
	return "wraps external CLI tools (whois, dig, theHarvester, sherlock, custom)"
}
func (s *Scanner) CanRun(t models.Target) bool {
	in := inputsFrom(t)
	for _, tool := range s.tools {
		if tool.applicable(in) {
			return true
		}
	}
	return false
}

type job struct {
	tool *Tool
	bin  string
	args []string // may contain dirPlaceholder when the tool uses ArgsDir
}

// Scan queues one job per tool invocation. Missing binaries are reported as
// error findings instead of failing the module.
func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	in := inputsFrom(t)

	var jobs []job
	for _, tool := range s.tools {
		if !tool.applicable(in) {
			continue
		}
		bin, err := exec.LookPath(tool.Binary)
		if err != nil {
			emit(models.Finding{Source: tool.Name, Category: "tooling", Status: models.StatusError,
				Details: map[string]string{"error": tool.Binary + " not installed or not in PATH"}})
			continue
		}
		if tool.ArgsDir != nil {
			for _, args := range tool.ArgsDir(in, dirPlaceholder) {
				jobs = append(jobs, job{tool, bin, args})
			}
			continue
		}
		for _, args := range tool.Args(in) {
			jobs = append(jobs, job{tool, bin, args})
		}
	}

	var (
		wg   sync.WaitGroup
		done atomic.Int64
		sem  = make(chan struct{}, s.cfg.MaxProcs)
	)
	for _, j := range jobs {
		wg.Add(1)
		err := d.Pool.Submit(ctx, func() {
			defer wg.Done()
			defer func() {
				if d.Progress != nil {
					d.Progress(s.Name(), int(done.Add(1)), len(jobs))
				}
			}()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			s.run(ctx, in, j, emit)
		})
		if err != nil {
			wg.Done()
			break
		}
	}
	wg.Wait()
	return ctx.Err()
}

func (s *Scanner) run(ctx context.Context, in Inputs, j job, emit func(models.Finding)) {
	timeout := j.tool.Timeout
	if timeout <= 0 {
		timeout = time.Minute
	}
	if s.cfg.TimeoutCap > 0 && timeout > s.cfg.TimeoutCap {
		timeout = s.cfg.TimeoutCap
	}

	args, dir := j.args, ""
	if j.tool.ArgsDir != nil {
		d, derr := os.MkdirTemp("", "argus-"+j.tool.Name+"-")
		if derr != nil {
			emit(models.Finding{Source: j.tool.Name, Category: "tooling", Status: models.StatusError,
				Details: map[string]string{"error": "cannot create a temporary directory: " + derr.Error()}})
			return
		}
		defer os.RemoveAll(d)
		dir = d
		args = make([]string, len(j.args))
		for i, a := range j.args {
			args[i] = strings.ReplaceAll(a, dirPlaceholder, dir)
		}
	}
	res := runCmdFull(ctx, j.bin, args, timeout, s.cfg.MaxOutput)
	if ctx.Err() != nil {
		return
	}
	out, err := res.Stdout, res.Err

	// The complete output of every run is kept in the report, whatever the parser
	// makes of it. Files a tool wrote (Maigret's JSON report) are appended.
	full := out
	if dir != "" {
		full += readReportFiles(dir)
	}
	emit(models.Finding{Source: j.tool.Name, Category: "toolrun", Status: models.StatusFound, Details: map[string]string{
		"command": j.bin + " " + strings.Join(redactDir(args, dir), " "), "output": full, "stderr": res.Stderr,
		"bytes": strconv.Itoa(len(full)), "duration_ms": strconv.FormatInt(res.Duration.Milliseconds(), 10),
		"truncated": yesNo(res.Truncated), "error": errText(err),
	}})

	var findings []models.Finding
	if j.tool.ParseDir != nil {
		findings = j.tool.ParseDir(in, dir, out)
	} else {
		findings = j.tool.Parse(in, args, out)
	}
	for _, f := range findings {
		emit(f)
	}
	// A failed run that still produced parseable output counts as success;
	// one that produced nothing is reported so the operator knows why.
	if len(findings) == 0 && err != nil {
		emit(models.Finding{Source: j.tool.Name, Category: "tooling", Status: models.StatusError,
			Details: map[string]string{"error": err.Error()}})
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// redactDir shows the temporary directory as <tmp> in the recorded command line.
func redactDir(args []string, dir string) []string {
	if dir == "" {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strings.ReplaceAll(a, dir, "<tmp>")
	}
	return out
}

// readReportFiles returns the text files a tool wrote into dir, each under a header.
func readReportFiles(dir string) string {
	var b strings.Builder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(data) > 64<<20 {
			continue
		}
		fmt.Fprintf(&b, "\n\n===== file written by the tool: %s =====\n%s", e.Name(), data)
	}
	return b.String()
}
