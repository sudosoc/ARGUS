package syswrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// limitedBuf keeps at most max bytes so a runaway tool cannot exhaust memory;
// truncated records whether anything was dropped (it is then reported, never silent).
type limitedBuf struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (l *limitedBuf) Write(p []byte) (int, error) {
	rem := l.max - l.buf.Len()
	switch {
	case rem <= 0:
		l.truncated = true
	case len(p) > rem:
		l.buf.Write(p[:rem])
		l.truncated = true
	default:
		l.buf.Write(p)
	}
	return len(p), nil
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// cmdResult is everything a tool run produced.
type cmdResult struct {
	Stdout, Stderr string
	Truncated      bool
	Err            error
	Duration       time.Duration
}

// runCmdFull executes bin directly (never through a shell) with a hard timeout
// and keeps all of stdout (up to maxOut bytes) and stderr (up to 1 MiB).
// stdout is returned even when the command fails, because tools such as whois
// and dig exit non-zero yet still print useful data.
func runCmdFull(ctx context.Context, bin string, args []string, timeout time.Duration, maxOut int) cmdResult {
	return runCmdEnv(ctx, bin, args, nil, timeout, maxOut)
}

// runCmdEnv is runCmdFull with extra environment variables (API keys the settings page holds).
func runCmdEnv(ctx context.Context, bin string, args, env []string, timeout time.Duration, maxOut int) cmdResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = 2 * time.Second
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdout := &limitedBuf{max: maxOut}
	stderr := &limitedBuf{max: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdout, stderr

	start := time.Now()
	err := cmd.Run()
	res := cmdResult{
		Stdout:    ansiRe.ReplaceAllString(stdout.buf.String(), ""),
		Stderr:    ansiRe.ReplaceAllString(stderr.buf.String(), ""),
		Truncated: stdout.truncated,
		Duration:  time.Since(start),
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timed out after %s", timeout)
		} else if msg := strings.TrimSpace(res.Stderr); msg != "" {
			if len(msg) > 300 {
				msg = msg[:300]
			}
			err = fmt.Errorf("%w: %s", err, msg)
		}
		res.Err = err
	}
	return res
}

// runCmd is runCmdFull reduced to stdout and the error.
func runCmd(ctx context.Context, bin string, args []string, timeout time.Duration, maxOut int) (string, error) {
	r := runCmdFull(ctx, bin, args, timeout, maxOut)
	return r.Stdout, r.Err
}
