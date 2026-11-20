// Package cliprobe runs bounded diagnostic CLI commands and validates their output.
package cliprobe

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type Output struct {
	Stdout, Stderr []byte
	Exit           int
	Failure        string
}

// cappedOutput bounds memory while continuing to drain provider output. Nothing
// from these buffers is logged or copied to the report except validated fields.
type cappedOutput struct {
	data     bytes.Buffer
	overflow bool
}

func (b *cappedOutput) Bytes() []byte { return b.data.Bytes() }

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := 65536 - b.data.Len()
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	_, _ = b.data.Write(p)
	return n, nil
}

func Run(ctx context.Context, cli, dir string, env, args []string) Output {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Dir, cmd.Env = dir, env
	// Bound inherited pipes after direct-child exit/cancellation. This does not
	// claim to stop every detached descendant and is never session-unlock proof.
	cmd.WaitDelay = 500 * time.Millisecond
	platform.HideConsole(cmd)
	var stdout, stderr cappedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	out := Output{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Exit: -1}
	if cmd.ProcessState != nil {
		out.Exit = cmd.ProcessState.ExitCode()
	}
	switch {
	case ctx.Err() != nil:
		out.Failure = "timeout_or_cancelled"
	case stdout.overflow || stderr.overflow:
		out.Failure = "output_limit"
	case errors.Is(err, exec.ErrWaitDelay):
		out.Failure = "inherited_pipe_not_closed"
	case err != nil && out.Exit < 0:
		out.Failure = "execution_failed"
	}
	return out
}

const cliSemver = `[0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9}(?:-(?:alpha|beta|rc|dev)(?:\.[0-9]{1,9})?)?`

var versionPatterns = map[string]*regexp.Regexp{
	"claude": regexp.MustCompile(`^(` + cliSemver + `) \(Claude Code\)$`),
	"codex":  regexp.MustCompile(`^codex-cli (` + cliSemver + `)$`),
	"cursor": regexp.MustCompile(`^([0-9]{4}\.[0-9]{2}\.[0-9]{2}-[a-f0-9]{7,40})$`),
}

// Version reads the normalized version reported by a successful --version call.
func Version(provider string, out Output) (version, reason string) {
	if out.Failure != "" {
		return "", out.Failure
	}
	if pattern := versionPatterns[provider]; pattern != nil && out.Exit == 0 {
		match := pattern.FindStringSubmatch(strings.TrimSpace(string(out.Stdout)))
		if len(match) == 2 && len(match[1]) <= 64 {
			return match[1], ""
		}
	}
	return "", "unrecognized_version_output"
}
