package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 120 * time.Second

// ExecResult holds the output of a command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Run executes a shell command on the local system.
func Run(ctx context.Context, command, shell string, timeoutSec int) ExecResult {
	timeout := defaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch strings.ToLower(shell) {
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command)
	default:
		// PowerShell is the default
		cmd = exec.CommandContext(ctx, "powershell.exe",
			"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-Command", command,
		)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf("command timed out after %v: %s", timeout, result.Stderr)
		} else {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf("exec error: %v\n%s", err, result.Stderr)
		}
	}

	return result
}
